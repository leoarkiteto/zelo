// Package pagination holds the page math shared by every paginated list:
// parsing the `page` query value, clamping it, computing offsets, slicing
// in-memory results and building page links. It performs no I/O.
package pagination

import "net/url"

// PageSize is the number of rows shown per page.
const PageSize = 20

// Page describes one page of a list.
type Page struct {
	Number     int // 1-based, clamped
	Size       int // always PageSize
	Total      int // total rows under the filter
	TotalPages int // >= 1
}

// Parse converts a raw `page` value to a page number; empty, non-integer or
// values < 1 resolve to 1.
func Parse(raw string) int {
	n := 0
	if raw == "" {
		return 1
	}
	sign := 1
	s := raw
	if s[0] == '+' || s[0] == '-' {
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	if s == "" {
		return 1
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 1
		}
		n = n*10 + int(c-'0')
		if n > 1<<40 { // avoid overflow; clamped later by New
			n = 1 << 40
		}
	}
	n *= sign
	if n < 1 {
		return 1
	}
	return n
}

// New builds a Page, clamping requested into [1, TotalPages]. A negative
// total is treated as 0.
func New(requested, total int) Page {
	if total < 0 {
		total = 0
	}
	pages := (total + PageSize - 1) / PageSize
	if pages < 1 {
		pages = 1
	}
	n := requested
	if n < 1 {
		n = 1
	}
	if n > pages {
		n = pages
	}
	return Page{Number: n, Size: PageSize, Total: total, TotalPages: pages}
}

// Offset is the number of rows preceding this page.
func (p Page) Offset() int { return (p.Number - 1) * p.Size }

// HasPrev reports whether a previous page exists.
func (p Page) HasPrev() bool { return p.Number > 1 }

// HasNext reports whether a next page exists.
func (p Page) HasNext() bool { return p.Number < p.TotalPages }

// Slice returns the sub-slice of items for the clamped requested page.
func Slice[T any](items []T, requested int) ([]T, Page) {
	p := New(requested, len(items))
	start := p.Offset()
	end := start + p.Size
	if end > len(items) {
		end = len(items)
	}
	if start > end {
		start = end
	}
	return items[start:end], p
}

// URL returns path?query with param set to n, keeping every other param of q
// and dropping "flash". It does not mutate q.
func URL(path string, q url.Values, param string, n int) string {
	out := make(url.Values, len(q)+1)
	for k, v := range q {
		if k == "flash" || k == param {
			continue
		}
		out[k] = append([]string(nil), v...)
	}
	out.Set(param, itoa(n))
	return path + "?" + out.Encode()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
