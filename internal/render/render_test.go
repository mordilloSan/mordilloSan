package render

import (
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mordilloSan/mordilloSan/internal/profile"
)

func sampleProfile() profile.Profile {
	week := func(last int) []int {
		w := make([]int, 52)
		w[51], w[40] = last, last/2
		return w
	}
	at := func(s string) time.Time {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			panic(err)
		}
		return t
	}
	return profile.Profile{
		Login: "someone", Name: "Some One", Contributions: 1234, Now: at("2026-11-16"),
		Repos: []profile.Repo{
			{Name: "big", Description: "The main project <with> \"odd\" & characters", Language: "Go", URL: "https://github.com/someone/big", Stars: 2, Commits: 2813, ReleaseTag: "v0.53.0", ReleasedAt: at("2026-11-13"), PushedAt: at("2026-11-16"), Weekly: week(165)},
			{Name: "a-rather-long-repository-name", Description: strings.Repeat("A long description that has to wrap and be cut. ", 4), Language: "JavaScript", URL: "https://github.com/someone/long", Commits: 31, ReleaseTag: "v1.7.0", ReleasedAt: at("2026-11-15"), PushedAt: at("2026-11-15"), Weekly: week(13)},
			{Name: "unreleased", URL: "https://github.com/someone/unreleased", Commits: 1, PushedAt: at("2026-06-01"), Weekly: make([]int, 52)},
			{Name: "old", Description: "Retired", Language: "C", URL: "https://github.com/someone/old", Stars: 6, Commits: 12, Archived: true, ReleaseTag: "v1.0.1", ReleasedAt: at("2026-11-11"), PushedAt: at("2026-11-11"), Weekly: week(12)},
		},
	}
}

func build(t *testing.T) (Page, profile.Layout) {
	t.Helper()
	p := sampleProfile()
	l := profile.Select(p.Repos, "", 4)
	return Build(p, l, "A tagline."), l
}

func wellFormed(t *testing.T, name, doc string) {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(doc))
	for {
		if _, err := d.Token(); err != nil {
			if !errors.Is(err, io.EOF) {
				t.Errorf("%s is not well-formed XML: %v", name, err)
			}
			return
		}
	}
}

func TestEveryPieceIsWellFormedSVG(t *testing.T) {
	pg, _ := build(t)
	th := Theme{Accent: "#ff8a3d", AccentLight: "#c2540a"}
	for _, p := range pg.All() {
		svg := p.SVG(th, ModeAuto)
		wellFormed(t, p.File, svg)
		if strings.Contains(svg, "NaN") || strings.Contains(svg, "Inf") {
			t.Errorf("%s contains a non-finite coordinate", p.File)
		}
	}
	for _, m := range []Mode{ModeDark, ModeLight} {
		wellFormed(t, "sheet", pg.Sheet(th, m))
	}
}

func TestPageHasOnePieceForEachSlot(t *testing.T) {
	pg, l := build(t)
	if len(pg.Feature) != 2 || pg.Feature[0].Link != "https://github.com/someone/big" {
		t.Fatalf("feature = %+v, want the big repo", pg.Feature)
	}
	if len(pg.Cards) != len(l.Cards) {
		t.Errorf("cards = %d, want %d", len(pg.Cards), len(l.Cards))
	}
	if len(pg.Rows) != 4 {
		t.Errorf("table rows = %d, want one per repo (4)", len(pg.Rows))
	}
}

func TestReadmeLinksEveryRepository(t *testing.T) {
	pg, l := build(t)
	th := Theme{Accent: "#ff8a3d", AccentLight: "#c2540a"}
	readme := pg.Readme("assets", th)
	for _, r := range l.Table {
		if !strings.Contains(readme, `<a href="`+r.URL+`">`) {
			t.Errorf("README has no link to %s", r.Name)
		}
	}
	for _, p := range pg.All() {
		if !strings.Contains(readme, `src="assets/`+p.FileName(th)+`"`) {
			t.Errorf("README does not show %s", p.FileName(th))
		}
	}
	if strings.Contains(readme, `"odd"`) {
		t.Error("README alt text is not escaped")
	}
}

// A browser keeps an image for a few minutes, so a row that now shows a
// different repository has to be published under a new name, or the page
// shows the old row next to the new ones.
func TestImageNamesFollowTheirContent(t *testing.T) {
	pg, _ := build(t)
	th := Theme{Accent: "#ff8a3d", AccentLight: "#c2540a"}
	row := pg.Rows[0]
	name := row.FileName(th)
	want := regexp.MustCompile(`^` + regexp.QuoteMeta(strings.TrimSuffix(row.File, ".svg")) + `\.[0-9a-f]{8}\.svg$`)
	if !want.MatchString(name) {
		t.Errorf("FileName = %q, want %s", name, want)
	}
	if again := row.FileName(th); again != name {
		t.Errorf("FileName is not stable: %q then %q", name, again)
	}
	changed := row
	changed.Alt += " 1 star."
	if changed.FileName(th) == name {
		t.Error("FileName did not change with the image")
	}
	if row.FileName(Theme{Accent: "#000000", AccentLight: "#ffffff"}) == name {
		t.Error("FileName did not change with the theme")
	}
}

func TestHeroGrowsForATwoLineTagline(t *testing.T) {
	p := sampleProfile()
	l := profile.Select(p.Repos, "", 4)
	one := Build(p, l, "A tagline.").Hero.H
	for name, tagline := range map[string]string{
		"line break":      "First line.\nSecond line.",
		"too wide to fit": strings.Repeat("A tagline that runs on. ", 6),
	} {
		hero := Build(p, l, tagline).Hero
		if hero.H != one+21 {
			t.Errorf("%s: hero height = %v, want one more line (%v)", name, hero.H, one+21)
		}
		if strings.Contains(hero.Alt, "\n") {
			t.Errorf("%s: alt text keeps the line break: %q", name, hero.Alt)
		}
	}
}

func TestComma(t *testing.T) {
	for in, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 2813: "2,813", 1234567: "1,234,567"} {
		if got := comma(in); got != want {
			t.Errorf("comma(%d) = %q, want %q", in, got, want)
		}
	}
}
