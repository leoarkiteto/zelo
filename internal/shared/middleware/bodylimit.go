package middleware

import (
	"net/http"
	"strings"
)

// BodyLimitRule overrides the default request body limit for requests whose
// method matches and whose path matches Segments ("*" matches exactly one
// path segment).
type BodyLimitRule struct {
	Method   string
	Segments []string
	Bytes    int64
}

// BodyLimit wraps r.Body with http.MaxBytesReader using the first matching
// rule's limit, or defaultBytes. The CSRF middleware turns an exceeded limit
// into a 413 response.
func BodyLimit(defaultBytes int64, rules []BodyLimitRule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := defaultBytes
			segs := splitPath(r.URL.Path)
			for _, rule := range rules {
				if rule.Method == r.Method && matchSegments(rule.Segments, segs) {
					limit = rule.Bytes
					break
				}
			}
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func matchSegments(pattern, segs []string) bool {
	if len(pattern) != len(segs) {
		return false
	}
	for i, p := range pattern {
		if p != "*" && p != segs[i] {
			return false
		}
	}
	return true
}
