package storage

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The examples and their signatures come from the AWS S3 documentation for Signature Version 4: "Authenticating
// Requests: Using Query Parameters" and "Examples: Signature Calculations" (GET Object).
var (
	exampleCreds = credentials{accessKey: "AKIAIOSFODNN7EXAMPLE", secretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", region: "us-east-1"}
	exampleTime  = time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
)

func TestPresignMatchesTheAWSExample(t *testing.T) {
	u, _ := url.Parse("https://examplebucket.s3.amazonaws.com/test.txt")
	signed := exampleCreds.presign(http.MethodGet, u, nil, 24*time.Hour, exampleTime)

	q, _ := url.ParseQuery(signed.RawQuery)
	if got := q.Get("X-Amz-Signature"); got != "aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404" {
		t.Fatalf("signature %s", got)
	}
	if q.Get("X-Amz-Credential") != "AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request" || q.Get("X-Amz-SignedHeaders") != "host" {
		t.Fatalf("query %v", q)
	}
}

func TestHeaderSigningMatchesTheAWSGetObjectExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	exampleCreds.sign(req, emptyHash, exampleTime)

	auth := req.Header.Get("Authorization")
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request," +
		"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date," +
		"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if auth != want {
		t.Fatalf("authorization\n got %s\nwant %s", auth, want)
	}
}

func TestAPresignedPutSignsTheSizeAndTheType(t *testing.T) {
	u, _ := url.Parse("https://account.r2.cloudflarestorage.com/uploads/u/abc")
	creds := credentials{accessKey: "key", secretKey: "secret", region: "auto"}
	signed := creds.presign(http.MethodPut, u, map[string]string{"Content-Length": "512", "Content-Type": "text/plain"}, 10*time.Minute, exampleTime)

	q, _ := url.ParseQuery(signed.RawQuery)
	if q.Get("X-Amz-SignedHeaders") != "content-length;content-type;host" || q.Get("X-Amz-Expires") != "600" {
		t.Fatalf("query %v", q)
	}
	// Another size gives another signature, so the bucket refuses a file of another size.
	other := creds.presign(http.MethodPut, u, map[string]string{"Content-Length": "513", "Content-Type": "text/plain"}, 10*time.Minute, exampleTime)
	if strings.EqualFold(signed.Query().Get("X-Amz-Signature"), other.Query().Get("X-Amz-Signature")) {
		t.Fatal("the size is not in the signature")
	}
}
