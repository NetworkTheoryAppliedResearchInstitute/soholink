package httpapi

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
)

// scrubRecorder captures the status code, headers, and body of an HTTP
// response so that the errorScrubMiddleware can decide whether to forward the
// original response or replace it with an opaque error (T-005).
type scrubRecorder struct {
	w       http.ResponseWriter
	status  int
	body    bytes.Buffer
	headers http.Header
	wrote   bool
}

func newScrubRecorder(w http.ResponseWriter) *scrubRecorder {
	return &scrubRecorder{
		w:       w,
		headers: make(http.Header),
		status:  http.StatusOK,
	}
}

// Header returns the captured header map.  Handlers write to this map;
// flush() copies it to the real writer before sending the response.
func (r *scrubRecorder) Header() http.Header {
	return r.headers
}

// WriteHeader records the status code; the actual WriteHeader call on the
// underlying writer is deferred until flush().
func (r *scrubRecorder) WriteHeader(status int) {
	r.status = status
	r.wrote = true
}

// Write buffers the response body without forwarding it.
func (r *scrubRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.body.Write(b)
}

// flush copies the captured headers, status, and body to the real writer.
// Call exactly once after the handler returns.
func (r *scrubRecorder) flush() {
	dst := r.w.Header()
	for k, vs := range r.headers {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	r.w.WriteHeader(r.status)
	_, _ = r.w.Write(r.body.Bytes())
}

// errorScrubMiddleware is the outermost middleware in the HTTP handler chain.
// It intercepts any 5xx response and replaces the body with a structured,
// opaque JSON error containing only the HTTP status code and a request ID.
// The full internal error (stack traces, file paths, SQL errors, etc.) is
// logged at ERROR level for operator visibility but never sent to the client.
//
// This prevents information leakage that would aid reconnaissance or further
// attacks against the node (T-005: Information Disclosure via error responses).
//
// NOTE: WebSocket upgrade paths bypass the normal WriteHeader/Write flow via
// http.Hijack and are unaffected by this middleware.
func errorScrubMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := newScrubRecorder(w)
		next.ServeHTTP(rec, r)

		if rec.status >= 500 {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = "unknown"
			}
			log.Printf("[httpapi] T-005 internal error scrubbed (req=%s status=%d method=%s path=%s): %s",
				requestID, rec.status, r.Method, r.URL.Path, rec.body.String())
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(rec.status)
			fmt.Fprintf(w, `{"error":"internal_error","request_id":%q}`, requestID)
			return
		}

		rec.flush()
	})
}
