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

// DefaultBodyLimitBytes is the request body limit applied when no rule matches.
const DefaultBodyLimitBytes int64 = 1 << 20

// DefaultBodyLimitRules returns the application's per-route body limit
// overrides (multipart uploads). It is the single definition shared by the
// composition root and the integration harness.
func DefaultBodyLimitRules() []BodyLimitRule {
	return []BodyLimitRule{
		{Method: http.MethodPost, Segments: []string{"finance"}, Bytes: 6 << 20},
		{Method: http.MethodPost, Segments: []string{"finance", "*", "edit"}, Bytes: 6 << 20},
		{Method: http.MethodPost, Segments: []string{"invoices", "*", "attachments"}, Bytes: 52 << 20},
	}
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
