// Package text turns strings into SVG outlines. GitHub shows README images
// without loading web fonts, so every letter is drawn as a path.
package text

import (
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/mordilloSan/mordilloSan/fonts"
)

// Face is one font file.
type Face struct {
	f    *sfnt.Font
	upem float64
	ppem fixed.Int26_6
}

// The faces used on the page.
var (
	Mono     = load("JetBrainsMono-Regular.ttf")
	MonoBold = load("JetBrainsMono-Bold.ttf")
	Sans     = load("Inter-Regular.ttf")
	Display  = load("SpaceGrotesk-Bold.ttf")
)

func load(name string) *Face {
	b, err := fonts.FS.ReadFile(name)
	if err != nil {
		panic(err)
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		panic(name + ": " + err.Error())
	}
	upem := f.UnitsPerEm()
	return &Face{f: f, upem: float64(upem), ppem: fixed.I(int(upem))}
}

// walk lays out s and calls emit with each glyph and its x offset. It returns
// the width of the run. tracking is extra space after every glyph, in pixels.
func (f *Face) walk(s string, size, tracking float64, emit func(sfnt.GlyphIndex, float64)) float64 {
	var buf sfnt.Buffer
	k := size / f.upem / 64
	x := 0.0
	var prev sfnt.GlyphIndex
	first := true
	for _, r := range s {
		idx, err := f.f.GlyphIndex(&buf, r)
		if err != nil || idx == 0 {
			idx, _ = f.f.GlyphIndex(&buf, '?')
		}
		if !first {
			if kern, err := f.f.Kern(&buf, prev, idx, f.ppem, font.HintingNone); err == nil {
				x += float64(kern) * k
			}
			x += tracking
		}
		if emit != nil {
			emit(idx, x)
		}
		if adv, err := f.f.GlyphAdvance(&buf, idx, f.ppem, font.HintingNone); err == nil {
			x += float64(adv) * k
		}
		prev, first = idx, false
	}
	return x
}

// Width is the width of s in pixels at the given size.
func (f *Face) Width(s string, size, tracking float64) float64 {
	return f.walk(s, size, tracking, nil)
}

// Path returns SVG path data for s with its baseline starting at (x, y).
func (f *Face) Path(s string, x, y, size, tracking float64) string {
	var sb strings.Builder
	var buf sfnt.Buffer
	k := size / f.upem / 64
	f.walk(s, size, tracking, func(idx sfnt.GlyphIndex, gx float64) {
		segs, err := f.f.LoadGlyph(&buf, idx, f.ppem, nil)
		if err != nil {
			return
		}
		pt := func(p fixed.Point26_6) {
			sb.WriteString(num(x + gx + float64(p.X)*k))
			sb.WriteByte(' ')
			sb.WriteString(num(y + float64(p.Y)*k))
		}
		open := false
		for _, sg := range segs {
			switch sg.Op {
			case sfnt.SegmentOpMoveTo:
				if open {
					sb.WriteByte('Z')
				}
				open = true
				sb.WriteByte('M')
				pt(sg.Args[0])
			case sfnt.SegmentOpLineTo:
				sb.WriteByte('L')
				pt(sg.Args[0])
			case sfnt.SegmentOpQuadTo:
				sb.WriteByte('Q')
				pt(sg.Args[0])
				sb.WriteByte(' ')
				pt(sg.Args[1])
			case sfnt.SegmentOpCubeTo:
				sb.WriteByte('C')
				pt(sg.Args[0])
				sb.WriteByte(' ')
				pt(sg.Args[1])
				sb.WriteByte(' ')
				pt(sg.Args[2])
			}
		}
		if open {
			sb.WriteByte('Z')
		}
	})
	return sb.String()
}

// Fit shortens s with an ellipsis until it is no wider than maxW.
func (f *Face) Fit(s string, size, tracking, maxW float64) string {
	if f.Width(s, size, tracking) <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		t := strings.TrimRight(string(rs), " .,;:") + "…"
		if f.Width(t, size, tracking) <= maxW {
			return t
		}
	}
	return ""
}

// Wrap breaks s into at most maxLines lines no wider than maxW. A newline in
// s starts a new line. Text that does not fit ends with an ellipsis.
func (f *Face) Wrap(s string, size, maxW float64, maxLines int) []string {
	var lines []string
	for para := range strings.SplitSeq(s, "\n") {
		cur := ""
		for w := range strings.FieldsSeq(para) {
			if next := cur + " " + w; cur != "" && f.Width(next, size, 0) <= maxW {
				cur = next
				continue
			}
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = w
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines-1], strings.Join(lines[maxLines-1:], " "))
	}
	for i, l := range lines {
		lines[i] = f.Fit(l, size, 0, maxW)
	}
	return lines
}

// num formats a coordinate with at most two decimals.
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		return "0"
	}
	return s
}
