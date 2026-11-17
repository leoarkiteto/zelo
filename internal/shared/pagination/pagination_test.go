package pagination

import (
	"net/url"
	"testing"
)

func TestParse(t *testing.T) {
	cases := map[string]int{
		"": 1, "abc": 1, "0": 1, "-3": 1, "1": 1, "7": 7, "2.5": 1, "+": 1, "-": 1,
		"99999999999999999999999": 1 << 40,
	}
	for in, want := range cases {
		if got := Parse(in); got != want {
			t.Errorf("Parse(%q)=%d want %d", in, got, want)
		}
	}
}

func TestNew(t *testing.T) {
	cases := []struct {
		req, total, num, pages int
	}{
		{1, 0, 1, 1}, {5, 0, 1, 1}, {0, 10, 1, 1}, {-2, 10, 1, 1},
		{1, 20, 1, 1}, {2, 20, 1, 1}, {1, 21, 1, 2}, {2, 21, 2, 2},
		{9, 21, 2, 2}, {3, 100, 3, 5}, {1, -5, 1, 1},
	}
	for _, c := range cases {
		p := New(c.req, c.total)
		if p.Number != c.num || p.TotalPages != c.pages || p.Size != PageSize {
			t.Errorf("New(%d,%d)=%+v want num %d pages %d", c.req, c.total, p, c.num, c.pages)
		}
	}
	if New(1, -5).Total != 0 {
		t.Error("negative total should be 0")
	}
}

func TestOffsetPrevNext(t *testing.T) {
	p := New(2, 45)
	if p.Offset() != 20 || !p.HasPrev() || !p.HasNext() {
		t.Errorf("middle page: %+v", p)
	}
	p = New(1, 45)
	if p.Offset() != 0 || p.HasPrev() || !p.HasNext() {
		t.Errorf("first page: %+v", p)
	}
	p = New(3, 45)
	if p.Offset() != 40 || !p.HasPrev() || p.HasNext() {
		t.Errorf("last page: %+v", p)
	}
}

func TestSlice(t *testing.T) {
	mk := func(n int) []int {
		s := make([]int, n)
		for i := range s {
			s[i] = i
		}
		return s
	}
	cases := []struct {
		n, req, wantLen, wantPages, wantNum, first int
	}{
		{0, 1, 0, 1, 1, -1},
		{20, 1, 20, 1, 1, 0},
		{21, 1, 20, 2, 1, 0},
		{21, 2, 1, 2, 2, 20},
		{21, 99, 1, 2, 2, 20},
		{50, 3, 10, 3, 3, 40},
		{50, 0, 20, 3, 1, 0},
	}
	for _, c := range cases {
		got, p := Slice(mk(c.n), c.req)
		if len(got) != c.wantLen || p.TotalPages != c.wantPages || p.Number != c.wantNum {
			t.Errorf("Slice(n=%d,req=%d): len=%d page=%+v", c.n, c.req, len(got), p)
		}
		if c.wantLen > 0 && got[0] != c.first {
			t.Errorf("Slice(n=%d,req=%d): first=%d want %d", c.n, c.req, got[0], c.first)
		}
	}
}

func TestURL(t *testing.T) {
	q := url.Values{"status": {"open"}, "flash": {"saved"}, "page": {"2", "5"}, "a b": {"x&y"}}
	got := URL("/finance", q, "page", 3)
	want := "/finance?a+b=x%26y&page=3&status=open"
	if got != want {
		t.Errorf("URL=%q want %q", got, want)
	}
	if len(q["page"]) != 2 || q.Get("flash") != "saved" || len(q) != 4 {
		t.Errorf("q mutated: %v", q)
	}
	if got := URL("/x", nil, "p", 1); got != "/x?p=1" {
		t.Errorf("nil q: %q", got)
	}
}
