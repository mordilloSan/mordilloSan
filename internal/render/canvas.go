// Package render draws the pieces of the profile page as SVG images and
// writes the README that stacks them.
package render

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/mordilloSan/mordilloSan/internal/text"
)

// Theme is the part of the look that is configurable.
type Theme struct {
	Accent      string // data colour on the dark variant
	AccentLight string // data colour on the light variant
}

// Piece is one finished image.
type Piece struct {
	File string  // file name under assets/
	W, H float64 // size in design pixels
	Alt  string  // text alternative
	Link string  // where a click goes; empty for no link
	body string
}

// Mode forces one colour variant; ModeAuto follows the viewer's system.
type Mode int

const (
	ModeAuto Mode = iota
	ModeDark
	ModeLight
)

const (
	darkVars  = "--bg:#12151a;--bd:#2a303a;--ln:#232830;--t:#eef0f2;--m:#98a1ad;--o:#f4ead8"
	lightVars = "--bg:#ffffff;--bd:#d5d9df;--ln:#e3e6ea;--t:#15181c;--m:#5a6370;--o:#3a4048"
	rules     = ".t{fill:var(--t)}.m{fill:var(--m)}.a{fill:var(--a)}.o{fill:var(--o)}" +
		".bg{fill:var(--bg);stroke:var(--bd)}.bgf{fill:var(--bg);stroke:var(--a)}" +
		".sa{stroke:var(--a);fill:none}.so{stroke:var(--o);fill:none}.sl{stroke:var(--ln);fill:none}"
)

func style(th Theme, m Mode) string {
	dark := ":root{" + darkVars + ";--a:" + th.Accent + "}"
	light := ":root{" + lightVars + ";--a:" + th.AccentLight + "}"
	switch m {
	case ModeDark:
		return "<style>" + dark + rules + "</style>"
	case ModeLight:
		return "<style>" + light + rules + "</style>"
	}
	return "<style>" + dark + "@media (prefers-color-scheme:light){" + light + "}" + rules + "</style>"
}

// SVG returns the piece as a standalone image.
func (p Piece) SVG(th Theme, m Mode) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" role="img" aria-label="%s">%s%s</svg>`+"\n",
		n(p.W), n(p.H), n(p.W), n(p.H), html.EscapeString(p.Alt), style(th, m), p.body)
}

type canvas struct {
	w, h float64
	sb   strings.Builder
}

func newCanvas(w, h float64) *canvas { return &canvas{w: w, h: h} }

func (c *canvas) piece(file, alt, link string) Piece {
	return Piece{File: file, W: c.w, H: c.h, Alt: alt, Link: link, body: c.sb.String()}
}

// slice returns the band of p that starts at y and is h tall, as its own
// image. Stacked in order, the slices show p again.
func (p Piece) slice(file, alt string, y, h float64) Piece {
	p.File, p.Alt, p.H = file, alt, h
	p.body = fmt.Sprintf(`<g transform="translate(0 %s)">%s</g>`, n(-y), p.body)
	return p
}

// tile draws the rounded background panel over the whole canvas.
func (c *canvas) tile(rx float64, class string) { c.box(0, 0, c.w, c.h, rx, class) }

// box draws a rounded panel that fills the given area.
func (c *canvas) box(x, y, w, h, rx float64, class string) {
	fmt.Fprintf(&c.sb, `<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="%s"/>`, class, n(x+.5), n(y+.5), n(w-1), n(h-1), n(rx))
}

// text draws s with its baseline starting at (x, y) and returns its width.
func (c *canvas) text(f *text.Face, s string, x, y, size, tracking float64, class string) float64 {
	if s == "" {
		return 0
	}
	fmt.Fprintf(&c.sb, `<path class="%s" d="%s"/>`, class, f.Path(s, x, y, size, tracking))
	return f.Width(s, size, tracking)
}

// textR draws s ending at x and returns its width.
func (c *canvas) textR(f *text.Face, s string, x, y, size, tracking float64, class string) float64 {
	w := f.Width(s, size, tracking)
	c.text(f, s, x-w, y, size, tracking, class)
	return w
}

func (c *canvas) rect(x, y, w, h float64, class string) {
	fmt.Fprintf(&c.sb, `<rect class="%s" x="%s" y="%s" width="%s" height="%s"/>`, class, n(x), n(y), n(w), n(h))
}

func (c *canvas) stroke(d, class string, width float64, dash string) {
	if d == "" {
		return
	}
	extra := ""
	if dash != "" {
		extra = ` stroke-dasharray="` + dash + `"`
	}
	fmt.Fprintf(&c.sb, `<path class="%s" stroke-width="%s"%s d="%s"/>`, class, n(width), extra, d)
}

const starPath = "M12 2l3.1 6.6 7.2.9-5.3 5 1.4 7.1L12 18l-6.4 3.6 1.4-7.1-5.3-5 7.2-.9z"

// starsWidth is the room a "(6★)" marker takes, including its leading gap.
func starsWidth(count int, size float64) float64 {
	if count <= 0 {
		return 0
	}
	return size*0.5 + text.Mono.Width("("+strconv.Itoa(count), size, 0) + 1 + size*0.85 + 1 + text.Mono.Width(")", size, 0)
}

// stars draws a "(6★)" marker starting at x, on the baseline y.
func (c *canvas) stars(count int, x, y, size float64) {
	if count <= 0 {
		return
	}
	x += size * 0.5
	x += c.text(text.Mono, "("+strconv.Itoa(count), x, y, size, 0, "m") + 1
	s := size * 0.85
	fmt.Fprintf(&c.sb, `<path class="a" transform="translate(%s %s) scale(%s)" d="%s"/>`, n(x), n(y-s*0.92), n(s/24), starPath)
	c.text(text.Mono, ")", x+s+1, y, size, 0, "m")
}

// bars draws one thin bar per week, scaled to the tallest week.
func (c *canvas) bars(weekly []int, x0, w, base, h, width float64) {
	peak := 0
	for _, v := range weekly {
		peak = max(peak, v)
	}
	c.stroke(fmt.Sprintf("M%s %sH%s", n(x0), n(base+.5), n(x0+w)), "sl", 1, "")
	if peak == 0 || len(weekly) == 0 {
		return
	}
	step := w / float64(len(weekly))
	var d strings.Builder
	for k, v := range weekly {
		if v <= 0 {
			continue
		}
		bar := max(1.5, float64(v)/float64(peak)*h)
		fmt.Fprintf(&d, "M%s %sV%s", n(x0+step*float64(k)+step/2), n(base), n(base-bar))
	}
	c.stroke(d.String(), "sa", width, "")
}

// shrink returns the largest size, from size down to floor, at which s fits.
func shrink(f *text.Face, s string, size, floor, maxW float64) float64 {
	for size > floor && f.Width(s, size, 0) > maxW {
		size -= 0.5
	}
	return size
}

func n(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// comma formats 2813 as "2,813".
func comma(v int) string {
	s := strconv.Itoa(v)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func day(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("Jan 2")
}

func upper(s string) string { return strings.ToUpper(s) }

func plural(v int, one, many string) string {
	if v == 1 {
		return "1 " + one
	}
	return comma(v) + " " + many
}
