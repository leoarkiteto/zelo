package templates

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func TestNiceScale(t *testing.T) {
	cases := []struct {
		name         string
		lo, hi       int64
		wantMin, max int64
	}{
		{"all zero", 0, 0, 0, 1000},
		{"positive", 0, 123456, 0, 150000},
		{"negative included", -4200, 10000, -5000, 10000},
		{"only negative", -9000, -100, -10000, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NiceScale(c.lo, c.hi)
			if s.Min != c.wantMin || s.Max != c.max {
				t.Fatalf("got [%d,%d], want [%d,%d]", s.Min, s.Max, c.wantMin, c.max)
			}
			if s.Min > c.lo || s.Max < c.hi {
				t.Fatalf("scale does not cover data")
			}
			zero := false
			for _, tk := range s.Ticks {
				if tk == 0 {
					zero = true
				}
			}
			if !zero {
				t.Fatalf("ticks %v lack a zero baseline", s.Ticks)
			}
			if s.Ticks[0] != s.Min || s.Ticks[len(s.Ticks)-1] != s.Max {
				t.Fatalf("ticks %v do not span scale", s.Ticks)
			}
		})
	}
}

func TestScaleY(t *testing.T) {
	s := NiceScale(-5000, 10000)
	if s.Y(s.Max) >= s.Y(0) || s.Y(0) >= s.Y(s.Min) {
		t.Fatalf("y must decrease as value grows: %v %v %v", s.Y(s.Max), s.Y(0), s.Y(s.Min))
	}
	if math.Abs(s.Y(s.Min)-(ChartH-plotBottom)) > 1e-9 {
		t.Fatalf("min not at the plot bottom")
	}
}

func TestBars(t *testing.T) {
	c := Bars([]int64{10000, 0, 5000}, []int64{2500, 0, 7500})
	if len(c.Groups) != 3 {
		t.Fatalf("groups = %d", len(c.Groups))
	}
	g := c.Groups[0]
	if g.In.H <= g.Out.H {
		t.Fatalf("in bar should be taller: %v vs %v", g.In.H, g.Out.H)
	}
	if g.In.X >= g.Out.X {
		t.Fatalf("in bar should sit left of out bar")
	}
	if math.Abs(g.In.Y+g.In.H-c.ZeroY) > 1e-9 {
		t.Fatalf("bar must rest on the zero baseline")
	}
	z := c.Groups[1]
	if z.In.H != 0 || z.Out.H != 0 {
		t.Fatalf("zero month must have zero-height bars")
	}
	if c.Groups[0].LabelX >= c.Groups[1].LabelX {
		t.Fatalf("groups must advance left to right")
	}
	if Bars(nil, nil).Groups != nil {
		t.Fatalf("empty input yields no groups")
	}
}

func TestLine(t *testing.T) {
	c := Line([]int64{5000, -3000, 0})
	if len(c.Points) != 3 {
		t.Fatalf("points = %d", len(c.Points))
	}
	if c.Points[0].Y >= c.ZeroY || c.Points[1].Y <= c.ZeroY {
		t.Fatalf("positive above and negative below the zero line")
	}
	if c.Points[0].Negative || !c.Points[1].Negative || c.Points[2].Negative {
		t.Fatalf("negative flags wrong: %+v", c.Points)
	}
	if !strings.HasPrefix(c.Path, "M") || strings.Count(c.Path, "L") != 2 {
		t.Fatalf("path = %q", c.Path)
	}
	if Line(nil).Path != "" {
		t.Fatalf("empty line has no path")
	}
}

func TestCompactMoney(t *testing.T) {
	cases := []struct {
		loc  i18n.Language
		in   int64
		want string
	}{
		{i18n.LanguagePTBR, 0, "R$ 0"},
		{i18n.LanguagePTBR, 150000, "R$ 1,5k"},
		{i18n.LanguageEN, 150000, "R$ 1.5k"},
		{i18n.LanguageEN, -500000, "-R$ 5k"},
		{i18n.LanguageEN, 250000000, "R$ 2.5M"},
	}
	for _, c := range cases {
		if got := compactMoney(context.Background(), c.loc, c.in); got != c.want {
			t.Errorf("compactMoney(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
