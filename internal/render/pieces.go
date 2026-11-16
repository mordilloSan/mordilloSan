package render

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mordilloSan/mordilloSan/internal/profile"
	"github.com/mordilloSan/mordilloSan/internal/text"
)

// Design widths. The README shows the hero and table at 100%, the feature
// card at 40% and the small cards at 28%, so all pieces share one scale.
const (
	pageW    = 850.0
	featureW = 340.0
	cardW    = 238.0
	cardH    = 172.0
	featureH = 2*cardH + 6
	rows     = 17 // tallest column of the year graph, in blocks
)

// Page is every image of the profile page.
type Page struct {
	Hero    Piece
	Feature *Piece
	Cards   []Piece
	Head    Piece
	Rows    []Piece
}

// Build draws the whole page.
func Build(p profile.Profile, l profile.Layout, tagline string) Page {
	pg := Page{Hero: hero(p, l, tagline), Head: tableHead(len(l.Table))}
	if l.Feature != nil {
		f := feature(*l.Feature)
		pg.Feature = &f
	}
	for i, r := range l.Cards {
		pg.Cards = append(pg.Cards, card(r, i+1))
	}
	for i, r := range l.Table {
		pg.Rows = append(pg.Rows, tableRow(r, i+1))
	}
	return pg
}

func hero(p profile.Profile, l profile.Layout, tagline string) Piece {
	const left, right = 28.0, pageW - 28.0

	weeks := 52
	totals := profile.WeeklyTotals(p.Repos, weeks)
	active := profile.ActiveWeeks(totals)

	// Place the figures first, from the right: the name and tagline have to
	// stay clear of them, and the tagline's line count sets the height.
	type stat struct{ num, sub, l1, l2 string }
	stats := []stat{
		{comma(p.Contributions), "", "CONTRIBUTIONS", "IN 12 MONTHS"},
		{fmt.Sprint(active), fmt.Sprintf("/ %d", weeks), "WEEKS WITH", "A COMMIT"},
		{fmt.Sprint(len(p.Repos)), "", "PUBLIC", "REPOS"},
	}
	xs := make([]float64, len(stats))
	edge := right
	for i := len(stats) - 1; i >= 0; i-- {
		s := stats[i]
		full := text.Display.Width(s.num, 34, -0.5)
		if s.sub != "" {
			full += 5 + text.Display.Width(s.sub, 17, 0)
		}
		xs[i] = edge - max(full, text.Mono.Width(s.l1, 10, 0.6), text.Mono.Width(s.l2, 10, 0.6))
		edge = xs[i] - 30
	}
	// Up to two tagline lines; a line break in config.json picks where the
	// second starts. A second line pushes everything below it down.
	const tagLead = 21.0
	tag := text.Sans.Wrap(tagline, 15, edge-left, 2)
	dy := tagLead * float64(max(len(tag)-1, 0))

	c := newCanvas(pageW, 358+dy)
	c.tile(14, "bg")
	c.text(text.Mono, "GITHUB.COM/"+upper(p.Login), left, 40, 10.5, 0.7, "m")
	for i, s := range stats {
		nw := c.text(text.Display, s.num, xs[i], 96, 34, -0.5, "t")
		c.text(text.Display, s.sub, xs[i]+nw+5, 96, 17, 0, "m")
		c.text(text.Mono, s.l1, xs[i], 113, 10, 0.6, "m")
		c.text(text.Mono, s.l2, xs[i], 126, 10, 0.6, "m")
	}
	nameSize := shrink(text.Display, p.Name, 68, 30, (edge-left)/0.94)
	c.text(text.Display, p.Name, left-2, 106, nameSize, -nameSize*0.03, "t")
	for i, line := range tag {
		c.text(text.Sans, line, left, 142+tagLead*float64(i), 15, 0, "m")
	}

	// Year graph: one column of blocks per week, the featured repository in
	// the accent colour and everything else stacked on top.
	peak := 0
	for _, v := range totals {
		peak = max(peak, v)
	}
	unit := profile.BlockUnit(peak, rows)
	caption := fmt.Sprintf("COMMITS PER WEEK · LAST %d WEEKS · ONE BLOCK IS ABOUT %d COMMITS", weeks, unit)
	if unit == 1 {
		caption = fmt.Sprintf("COMMITS PER WEEK · LAST %d WEEKS · ONE BLOCK IS ONE COMMIT", weeks)
	}
	c.text(text.Mono, caption, left, 178+dy, 10, 0.6, "m")

	var featured []int
	if l.Feature != nil {
		featured = l.Feature.Weekly
		lx := right - text.Mono.Width("EVERYTHING ELSE", 10, 0.6)
		c.text(text.Mono, "EVERYTHING ELSE", lx, 178+dy, 10, 0.6, "m")
		c.rect(lx-15, 171+dy, 9, 5, "o")
		name := upper(l.Feature.Name)
		lx -= 15 + 18 + text.Mono.Width(name, 10, 0.6)
		c.text(text.Mono, name, lx, 178+dy, 10, 0.6, "m")
		c.rect(lx-15, 171+dy, 9, 5, "a")
	}

	const pitch = 7.0
	base := 318 + dy
	step := (right - left) / float64(weeks)
	var main, rest strings.Builder
	for k, tot := range totals {
		if tot <= 0 {
			continue
		}
		own := 0
		if off := k - (weeks - len(featured)); off >= 0 && off < len(featured) {
			own = featured[off]
		}
		// Split the column's blocks in proportion to the two shares.
		nT := profile.Blocks(tot, unit)
		nF := min(nT, int(math.Round(float64(own)/float64(tot)*float64(nT))))
		nO := nT - nF
		cx := left + step*float64(k) + step/2
		if nF > 0 {
			fmt.Fprintf(&main, "M%s %sV%s", n(cx), n(base), n(base-float64(nF)*pitch+2))
		}
		if nO > 0 {
			fmt.Fprintf(&rest, "M%s %sV%s", n(cx), n(base-float64(nF)*pitch), n(base-float64(nT)*pitch+2))
		}
	}
	c.stroke(fmt.Sprintf("M%s %sH%s", n(left), n(base+1.5), n(right)), "sl", 1, "")
	c.stroke(main.String(), "sa", 9, "5 2")
	c.stroke(rest.String(), "so", 9, "5 2")

	// Month labels. GitHub's weeks start on Sunday.
	sunday := p.Now.UTC().AddDate(0, 0, -int(p.Now.UTC().Weekday()))
	lastX, lastMonth := -100.0, time.Month(0)
	for k := range weeks {
		start := sunday.AddDate(0, 0, -7*(weeks-1-k))
		x := left + step*float64(k)
		if start.Month() != lastMonth && x-lastX >= 30 {
			c.text(text.Mono, upper(start.Format("Jan")), x, 341+dy, 10, 0.6, "m")
			lastX = x
		}
		lastMonth = start.Month()
	}

	alt := fmt.Sprintf("%s. %s %s contributions in the last 12 months, commits in %d of the last %d weeks, %s.",
		p.Name, strings.Join(strings.Fields(tagline), " "), comma(p.Contributions), active, weeks, plural(len(p.Repos), "public repo", "public repos"))
	return c.piece("hero.svg", alt, "")
}

func repoAlt(r profile.Repo) string {
	var b strings.Builder
	b.WriteString(r.Name)
	if r.Description != "" {
		b.WriteString(": " + strings.TrimRight(r.Description, ". ") + ".")
	}
	if r.Language != "" {
		b.WriteString(" " + r.Language + ".")
	}
	if r.ReleaseTag != "" {
		fmt.Fprintf(&b, " Latest release %s, %s.", r.ReleaseTag, day(r.ReleasedAt))
	}
	fmt.Fprintf(&b, " %s.", plural(r.Commits, "commit", "commits"))
	if r.Stars > 0 {
		fmt.Fprintf(&b, " %s.", plural(r.Stars, "star", "stars"))
	}
	if r.Archived {
		b.WriteString(" Archived.")
	}
	return b.String()
}

// title draws a repository name with its star count after it and the
// language at the right edge, shrinking the name when space is short.
func (c *canvas) title(r profile.Repo, left, right, y, size, floor, starSize, langSize float64) {
	langW := c.textR(text.Mono, upper(r.Language), right, y-1, langSize, 0.6, "m")
	if langW > 0 {
		langW += 10
	}
	size = shrink(text.MonoBold, r.Name, size, floor, right-left-langW-starsWidth(r.Stars, starSize))
	w := c.text(text.MonoBold, r.Name, left, y, size, 0, "t")
	c.stars(r.Stars, left+w, y, starSize)
}

func feature(r profile.Repo) Piece {
	const left, right = 18.0, featureW - 18.0
	c := newCanvas(featureW, featureH)
	c.tile(14, "bgf")
	c.title(r, left, right, 46, 26, 14, 12, 10)

	for i, line := range text.Sans.Wrap(r.Description, 13.5, right-left, 3) {
		c.text(text.Sans, line, left, 76+float64(i)*20, 13.5, 0, "m")
	}

	type stat struct{ num, label string }
	stats := []stat{{comma(r.Commits), "COMMITS"}}
	if r.ReleaseTag != "" {
		stats = append(stats, stat{r.ReleaseTag, "RELEASED " + upper(day(r.ReleasedAt))})
	}
	stats = append(stats, stat{day(r.PushedAt), "LAST PUSH"})
	x := left
	for _, s := range stats {
		w := max(text.Display.Width(s.num, 21, -0.3), text.Mono.Width(s.label, 9.5, 0.6))
		if x+w > right {
			break
		}
		c.text(text.Display, s.num, x, 178, 21, -0.3, "t")
		c.text(text.Mono, s.label, x, 193, 9.5, 0.6, "m")
		x += w + 20
	}

	year := r.YearCommits()
	c.text(text.Mono, fmt.Sprintf("LAST %d WEEKS · %s COMMITS", len(r.Weekly), comma(year)), left, 228, 10, 0.6, "m")
	c.bars(r.Weekly, left, right-left, featureH-20, 84, 3.6)
	return c.piece("feature.svg", repoAlt(r), r.URL)
}

func card(r profile.Repo, i int) Piece {
	const left, right = 14.0, cardW - 14.0
	c := newCanvas(cardW, cardH)
	c.tile(12, "bg")
	c.title(r, left, right, 30, 14, 10, 11, 9.5)
	c.text(text.Mono, upper(plural(r.Commits, "commit", "commits")), left, 47, 9.5, 0.6, "m")

	for k, line := range text.Sans.Wrap(r.Description, 12, right-left, 3) {
		c.text(text.Sans, line, left, 68+float64(k)*17, 12, 0, "m")
	}

	if r.ReleaseTag != "" {
		dw := c.textR(text.Mono, upper(day(r.ReleasedAt)), right, 132, 9.5, 0.6, "m")
		c.text(text.Display, text.Display.Fit(r.ReleaseTag, 19, -0.3, right-left-dw-10), left, 132, 19, -0.3, "t")
	} else {
		c.text(text.Mono, "NO RELEASE YET", left, 132, 9.5, 0.6, "m")
	}
	c.bars(r.Weekly, left, right-left, cardH-14, 18, 2.4)
	return c.piece(fmt.Sprintf("card-%d.svg", i), repoAlt(r), r.URL)
}

// Column positions shared by the table header and its rows.
const (
	colName    = 14.0
	colDesc    = 214.0
	colLang    = 520.0
	colRelease = 604.0
	colDate    = 684.0
	colCommits = 790.0 // right edge
	colStars   = 836.0 // right edge
)

func tableHead(count int) Piece {
	c := newCanvas(pageW, 46)
	c.text(text.Mono, fmt.Sprintf("ALL %s · NEWEST RELEASE FIRST", upper(plural(count, "repo", "repos"))), 2, 14, 10, 0.6, "m")
	const y = 38.0
	c.text(text.Mono, "REPO", colName, y, 9.5, 0.6, "m")
	c.text(text.Mono, "WHAT IT IS", colDesc, y, 9.5, 0.6, "m")
	c.text(text.Mono, "LANG", colLang, y, 9.5, 0.6, "m")
	c.text(text.Mono, "RELEASE", colRelease, y, 9.5, 0.6, "m")
	c.text(text.Mono, "DATE", colDate, y, 9.5, 0.6, "m")
	c.textR(text.Mono, "COMMITS", colCommits, y, 9.5, 0.6, "m")
	c.textR(text.Mono, "STARS", colStars, y, 9.5, 0.6, "m")
	return c.piece("table-head.svg", "", "")
}

func tableRow(r profile.Repo, i int) Piece {
	c := newCanvas(pageW, 34)
	c.tile(8, "bg")
	const y = 21.5
	strong := "t"
	if r.Archived {
		strong = "m"
	}
	tagW := 0.0
	if r.Archived {
		tagW = text.Mono.Width("ARCHIVED", 8.5, 0.6) + 8
	}
	name := text.MonoBold.Fit(r.Name, 12.5, 0, colDesc-colName-12-tagW)
	w := c.text(text.MonoBold, name, colName, y, 12.5, 0, strong)
	if r.Archived {
		c.text(text.Mono, "ARCHIVED", colName+w+8, y, 8.5, 0.6, "m")
	}
	c.text(text.Sans, text.Sans.Fit(r.Description, 12, 0, colLang-colDesc-16), colDesc, y, 12, 0, "m")
	c.text(text.Mono, text.Mono.Fit(r.Language, 11.5, 0, colRelease-colLang-8), colLang, y, 11.5, 0, "m")
	c.text(text.Mono, text.Mono.Fit(r.ReleaseTag, 11.5, 0, colDate-colRelease-10), colRelease, y, 11.5, 0, strong)
	c.text(text.Mono, day(r.ReleasedAt), colDate, y, 11.5, 0, "m")
	c.textR(text.Mono, comma(r.Commits), colCommits, y, 11.5, 0, strong)
	starClass := "m"
	if r.Stars > 0 {
		starClass = strong
	}
	c.textR(text.Mono, comma(r.Stars), colStars, y, 11.5, 0, starClass)
	return c.piece(fmt.Sprintf("row-%d.svg", i), repoAlt(r), r.URL)
}
