package templates

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// Fixed SVG viewBox and plot area, shared by both charts. The templates only
// place the shapes computed here.
const (
	ChartW = 720.0
	ChartH = 260.0

	plotLeft   = 64.0
	plotRight  = 12.0
	plotTop    = 12.0
	plotBottom = 30.0

	maxBarWidth = 22.0
)

// ChartViewBox is the viewBox attribute value of every overview chart.
func ChartViewBox() string { return fmt.Sprintf("0 0 %.0f %.0f", ChartW, ChartH) }

// Tick is one y-axis grid line.
type Tick struct {
	Value int64
	Y     float64
}

// Scale maps cent values to y coordinates. Min <= 0 <= Max always holds.
type Scale struct {
	Min, Max int64
	Ticks    []int64
}

// NiceScale returns a scale that includes zero and whose bounds and ticks are
// multiples of a "nice" step (1, 2, 5 x 10^k cents). It supports negative
// values. With no data (all zero) it spans 0..1000 cents.
func NiceScale(minV, maxV int64) Scale {
	lo, hi := minV, maxV
	if lo > 0 {
		lo = 0
	}
	if hi < 0 {
		hi = 0
	}
	if lo == 0 && hi == 0 {
		hi = 1000
	}
	step := niceStep(float64(hi-lo) / 4)
	lo = floorDiv(lo, step) * step
	hi = -floorDiv(-hi, step) * step
	var ticks []int64
	for v := lo; v <= hi; v += step {
		ticks = append(ticks, v)
	}
	return Scale{Min: lo, Max: hi, Ticks: ticks}
}

func niceStep(raw float64) int64 {
	if raw < 1 {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5, 10} {
		if m*mag >= raw {
			return int64(m * mag)
		}
	}
	return int64(10 * mag)
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// Y maps a value to its y coordinate inside the plot area (Max at the top).
func (s Scale) Y(v int64) float64 {
	top, bottom := plotTop, ChartH-plotBottom
	if s.Max == s.Min {
		return bottom
	}
	return top + (bottom-top)*float64(s.Max-v)/float64(s.Max-s.Min)
}

// TickLines returns the ticks with their y coordinates.
func (s Scale) TickLines() []Tick {
	out := make([]Tick, len(s.Ticks))
	for i, t := range s.Ticks {
		out[i] = Tick{Value: t, Y: s.Y(t)}
	}
	return out
}

// PlotLeft and PlotRight expose the horizontal plot bounds for grid lines.
func PlotLeft() float64  { return plotLeft }
func PlotRight() float64 { return ChartW - plotRight }

// Bar is one rectangle. H is always >= 0.
type Bar struct {
	X, Y, W, H float64
	Cents      int64
}

// BarGroup holds the entrada and saída bars of one month plus its label x.
type BarGroup struct {
	In, Out Bar
	LabelX  float64
}

// BarChart is the grouped-bar geometry.
type BarChart struct {
	Scale  Scale
	ZeroY  float64
	Groups []BarGroup
}

// Bars computes grouped bars: one group per index; ins[i]/outs[i] are the
// month's entradas/saídas in cents (outs is expected to have the same length).
func Bars(ins, outs []int64) BarChart {
	n := len(ins)
	lo, hi := int64(0), int64(0)
	for i := 0; i < n; i++ {
		for _, v := range []int64{ins[i], outs[i]} {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	sc := NiceScale(lo, hi)
	c := BarChart{Scale: sc, ZeroY: sc.Y(0)}
	if n == 0 {
		return c
	}
	slot := (ChartW - plotLeft - plotRight) / float64(n)
	bw := math.Min(maxBarWidth, slot*0.34)
	gap := math.Min(3, slot*0.04)
	for i := 0; i < n; i++ {
		cx := plotLeft + slot*(float64(i)+0.5)
		c.Groups = append(c.Groups, BarGroup{
			In:     c.bar(sc, cx-gap/2-bw, bw, ins[i]),
			Out:    c.bar(sc, cx+gap/2, bw, outs[i]),
			LabelX: cx,
		})
	}
	return c
}

func (c BarChart) bar(sc Scale, x, w float64, v int64) Bar {
	y, zero := sc.Y(v), c.ZeroY
	if v < 0 {
		return Bar{X: x, Y: zero, W: w, H: y - zero, Cents: v}
	}
	return Bar{X: x, Y: y, W: w, H: zero - y, Cents: v}
}

// LinePoint is one point of the balance line.
type LinePoint struct {
	X, Y     float64
	Cents    int64
	Negative bool
}

// LineChart is the balance-line geometry.
type LineChart struct {
	Scale  Scale
	ZeroY  float64
	Points []LinePoint
	Path   string // SVG "d" for the polyline, empty when there are no points
}

// Line computes the balance line; points are centred in the same slots as the
// grouped bars so both charts line up month by month.
func Line(vals []int64) LineChart {
	var lo, hi int64
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	sc := NiceScale(lo, hi)
	c := LineChart{Scale: sc, ZeroY: sc.Y(0)}
	n := len(vals)
	if n == 0 {
		return c
	}
	slot := (ChartW - plotLeft - plotRight) / float64(n)
	for i, v := range vals {
		p := LinePoint{X: plotLeft + slot*(float64(i)+0.5), Y: sc.Y(v), Cents: v, Negative: v < 0}
		c.Points = append(c.Points, p)
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		c.Path += fmt.Sprintf("%s%.1f %.1f", cmd, p.X, p.Y)
		if i < n-1 {
			c.Path += " "
		}
	}
	return c
}

// LabelY is the y of the month labels under the plot.
func LabelY() float64 { return ChartH - 10 }

// coord formats a coordinate for an SVG attribute.
func coord(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// compactMoney renders an axis label such as "R$ 1,5k" from cents, using the
// currency carried by ctx (BRL when unset). Symbol placement mirrors
// money.Format.
func compactMoney(ctx context.Context, locale i18n.Language, cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	v := float64(cents) / 100
	var num, suffix string
	switch {
	case v >= 1_000_000:
		num, suffix = strconv.FormatFloat(v/1_000_000, 'f', -1, 64), "M"
		num = trimDecimals(num)
	case v >= 1_000:
		num, suffix = trimDecimals(strconv.FormatFloat(v/1_000, 'f', -1, 64)), "k"
	default:
		num = trimDecimals(strconv.FormatFloat(v, 'f', -1, 64))
	}
	if locale == i18n.LanguagePTBR || locale == i18n.LanguagePTPT {
		for i := range num {
			if num[i] == '.' {
				num = num[:i] + "," + num[i+1:]
				break
			}
		}
	}
	if money.From(ctx) == money.EUR {
		switch locale {
		case i18n.LanguagePTPT:
			return sign + num + suffix + " €"
		case i18n.LanguagePTBR:
			return sign + "€ " + num + suffix
		default:
			return sign + "€" + num + suffix
		}
	}
	return sign + "R$ " + num + suffix
}

// trimDecimals keeps at most one decimal digit.
func trimDecimals(s string) string {
	for i := range s {
		if s[i] == '.' {
			if len(s) > i+2 {
				return s[:i+2]
			}
			return s
		}
	}
	return s
}
