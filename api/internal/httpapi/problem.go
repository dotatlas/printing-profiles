package httpapi

import (
	"encoding/json"
	"net/http"
)

// problem is an error answer in the RFC 9457 format. writeProblem sends it, and clients read it.
type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// writeProblem sends an error answer in the RFC 9457 format. Handlers and middleware use it for every error.
func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{Type: "about:blank", Title: http.StatusText(status), Status: status, Detail: detail})
}

// problemsForUnmatched sends the router's own 404 and 405 answers as problem details. Handler puts it around the router.
func problemsForUnmatched(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r) // A route matches. ServeHTTP, not h, also sets the path values.
			return
		}
		// No route matches. Run the router's own answer on a recorder to learn its status.
		rec := &headerRecorder{header: http.Header{}}
		h.ServeHTTP(rec, r)
		if rec.status < 400 {
			mux.ServeHTTP(w, r) // It is not an error, for example a redirect. Let the router send it.
			return
		}
		if allow := rec.header.Get("Allow"); allow != "" {
			w.Header().Set("Allow", allow) // A 405 tells the client which methods the path accepts.
		}
		writeProblem(w, rec.status, "")
	})
}

// headerRecorder keeps the status and headers of an answer and drops its body. problemsForUnmatched uses it.
type headerRecorder struct {
	header http.Header
	status int
}

// Header returns the headers that the router sets. It makes headerRecorder an http.ResponseWriter.
func (r *headerRecorder) Header() http.Header {
	return r.header
}

// Write drops the body and keeps status 200 when no status came first, as net/http does.
func (r *headerRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return len(b), nil
}

// WriteHeader keeps the first status that the router sends.
func (r *headerRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
}
