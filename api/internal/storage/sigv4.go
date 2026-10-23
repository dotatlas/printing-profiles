package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// emptyHash is the SHA-256 of an empty body, which a GET request signs.
const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// credentials sign S3 requests with AWS Signature Version 4, which R2 accepts. The bucket methods use them.
type credentials struct {
	accessKey, secretKey, region string
}

// hmacSHA256 gives the HMAC-SHA256 of data with key. The key derivation and the signature use it.
func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// sha256Hex gives the hex SHA-256 of data. The canonical request and the payload hash use it.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// scope gives the credential scope of a day: date/region/s3/aws4_request.
func (c credentials) scope(t time.Time) string {
	return t.Format("20060102") + "/" + c.region + "/s3/aws4_request"
}

// signature signs the canonical request with the key of its day and region. presign and sign use it.
func (c credentials) signature(canonical string, t time.Time) string {
	key := hmacSHA256([]byte("AWS4"+c.secretKey), t.Format("20060102"))
	key = hmacSHA256(key, c.region)
	key = hmacSHA256(key, "s3")
	key = hmacSHA256(key, "aws4_request")
	toSign := "AWS4-HMAC-SHA256\n" + t.Format("20060102T150405Z") + "\n" + c.scope(t) + "\n" + sha256Hex([]byte(canonical))
	return hex.EncodeToString(hmacSHA256(key, toSign))
}

// escape encodes a string the way Signature V4 wants: every byte except A-Z, a-z, 0-9, "-", "_", ".", "~" (and
// "/" in a path) becomes %XX.
func escape(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' || keepSlash && c == '/' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// canonicalQuery sorts and encodes query values for the canonical request.
func canonicalQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var parts []string
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, escape(k, false)+"="+escape(v, false))
		}
	}
	return strings.Join(parts, "&")
}

// canonicalRequest builds the text that Signature V4 signs, and the list of signed header names.
func canonicalRequest(method string, u *url.URL, query string, headers map[string]string, payloadHash string) (string, string) {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	slices.Sort(names)
	var lines strings.Builder
	for _, name := range names {
		lines.WriteString(name + ":" + strings.Join(strings.Fields(headers[name]), " ") + "\n")
	}
	signed := strings.Join(names, ";")
	path := u.Path
	if path == "" {
		path = "/"
	}
	return method + "\n" + escape(path, true) + "\n" + query + "\n" + lines.String() + "\n" + signed + "\n" + payloadHash, signed
}

// presign gives the URL that lets its holder send one request until ttl ends. The headers are part of the
// signature, so a presigned PUT with Content-Length and Content-Type allows only that size and that type.
func (c credentials) presign(method string, u *url.URL, headers map[string]string, ttl time.Duration, now time.Time) *url.URL {
	t := now.UTC()
	signHeaders := map[string]string{"host": u.Host}
	for name, value := range headers {
		signHeaders[strings.ToLower(name)] = value
	}
	names := make([]string, 0, len(signHeaders))
	for name := range signHeaders {
		names = append(names, name)
	}
	slices.Sort(names)
	q := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {c.accessKey + "/" + c.scope(t)},
		"X-Amz-Date":          {t.Format("20060102T150405Z")},
		"X-Amz-Expires":       {strconv.Itoa(int(ttl.Seconds()))},
		"X-Amz-SignedHeaders": {strings.Join(names, ";")},
	}
	query := canonicalQuery(q)
	canonical, _ := canonicalRequest(method, u, query, signHeaders, "UNSIGNED-PAYLOAD")
	out := *u
	out.RawQuery = query + "&X-Amz-Signature=" + c.signature(canonical, t)
	return &out
}

// sign adds the Signature V4 headers to a request that the API sends itself (a GET or a PUT of an object).
// payloadHash is the hex SHA-256 of the body.
func (c credentials) sign(req *http.Request, payloadHash string, now time.Time) {
	t := now.UTC()
	req.Header.Set("X-Amz-Date", t.Format("20060102T150405Z"))
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	headers := map[string]string{"host": host}
	for name, values := range req.Header {
		headers[strings.ToLower(name)] = strings.Join(values, ",")
	}
	canonical, signed := canonicalRequest(req.Method, req.URL, canonicalQuery(req.URL.Query()), headers, payloadHash)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.accessKey+"/"+c.scope(t)+",SignedHeaders="+signed+",Signature="+c.signature(canonical, t))
}
