package httpx

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// AuthOpLimit applies a tighter cap to login/register GraphQL operations.
func AuthOpLimit(perMin int, next http.Handler) http.Handler {
	authOnly := RateLimit(perMin, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && looksLikeAuthOp(r) {
			authOnly.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func looksLikeAuthOp(r *http.Request) bool {
	if r.Body == nil {
		return false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return false
	}
	q := strings.ToLower(string(raw))
	return strings.Contains(q, "mutation") && (strings.Contains(q, "login") || strings.Contains(q, "register"))
}
