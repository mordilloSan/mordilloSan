package profile

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func weeks(total int) []int {
	w := make([]int, 52)
	w[51] = total
	return w
}

func sample() []Repo {
	return []Repo{
		{Name: "LinuxIO", ReleaseTag: "v0.53.0", ReleasedAt: day("2026-11-13"), PushedAt: day("2026-11-16"), Weekly: weeks(2474)},
		{Name: "failover", ReleaseTag: "v2.9.0", ReleasedAt: day("2026-10-19"), PushedAt: day("2026-10-19"), Weekly: weeks(111)},
		{Name: "linux-toolbox", ReleaseTag: "v1.16.0", ReleasedAt: day("2026-11-11"), PushedAt: day("2026-11-15"), Weekly: weeks(107)},
		{Name: "gh-release-flow", ReleaseTag: "v1.7.0", ReleasedAt: day("2026-11-15"), PushedAt: day("2026-11-15"), Weekly: weeks(31)},
		{Name: "fan_control", ReleaseTag: "v1.4.0", ReleasedAt: day("2026-11-08"), PushedAt: day("2026-11-08"), Weekly: weeks(25)},
		{Name: "vscode-start-view", ReleaseTag: "v1.0.1", ReleasedAt: day("2026-11-11"), PushedAt: day("2026-11-11"), Weekly: weeks(12), Archived: true},
		{Name: "unifi-os-server", ReleaseTag: "v5.1.42-7", ReleasedAt: day("2026-09-25"), PushedAt: day("2026-11-15"), Weekly: weeks(6)},
	}
}

func names(rs []Repo) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSelectFeatureIsMostActiveRepo(t *testing.T) {
	l := Select(sample(), "", 4)
	if l.Feature == nil || l.Feature.Name != "LinuxIO" {
		t.Fatalf("feature = %v, want LinuxIO", l.Feature)
	}
}

func TestSelectFeatureOverride(t *testing.T) {
	l := Select(sample(), "fan_control", 4)
	if l.Feature == nil || l.Feature.Name != "fan_control" {
		t.Fatalf("feature = %v, want fan_control", l.Feature)
	}
}

func TestSelectFeatureOverrideIgnoresArchived(t *testing.T) {
	l := Select(sample(), "vscode-start-view", 4)
	if l.Feature == nil || l.Feature.Name != "LinuxIO" {
		t.Fatalf("feature = %v, want LinuxIO", l.Feature)
	}
}

func TestSelectCardsAreNewestReleasesWithoutFeatureOrArchived(t *testing.T) {
	l := Select(sample(), "", 4)
	want := []string{"gh-release-flow", "linux-toolbox", "fan_control", "failover"}
	if got := names(l.Cards); !equal(got, want) {
		t.Fatalf("cards = %v, want %v", got, want)
	}
}

func TestSelectCardsFillWithRecentlyPushedWhenFewReleases(t *testing.T) {
	repos := []Repo{
		{Name: "main", Weekly: weeks(50), PushedAt: day("2026-11-01")},
		{Name: "released", ReleaseTag: "v1", ReleasedAt: day("2026-01-01"), PushedAt: day("2026-01-01"), Weekly: weeks(1)},
		{Name: "old-push", PushedAt: day("2026-02-01"), Weekly: weeks(1)},
		{Name: "new-push", PushedAt: day("2026-06-01"), Weekly: weeks(1)},
	}
	l := Select(repos, "", 4)
	want := []string{"released", "new-push", "old-push"}
	if got := names(l.Cards); !equal(got, want) {
		t.Fatalf("cards = %v, want %v", got, want)
	}
}

func TestSelectTableListsEveryRepoArchivedLast(t *testing.T) {
	l := Select(sample(), "", 4)
	want := []string{"gh-release-flow", "LinuxIO", "linux-toolbox", "fan_control", "failover", "unifi-os-server", "vscode-start-view"}
	if got := names(l.Table); !equal(got, want) {
		t.Fatalf("table = %v, want %v", got, want)
	}
}

func TestSelectEmpty(t *testing.T) {
	l := Select(nil, "", 4)
	if l.Feature != nil || len(l.Cards) != 0 || len(l.Table) != 0 {
		t.Fatalf("expected empty layout, got %+v", l)
	}
}

func TestWeeklyTotalsAndActiveWeeks(t *testing.T) {
	a := Repo{Weekly: []int{1, 0, 3}}
	b := Repo{Weekly: []int{0, 0, 2}}
	tot := WeeklyTotals([]Repo{a, b}, 3)
	if tot[0] != 1 || tot[1] != 0 || tot[2] != 5 {
		t.Fatalf("totals = %v", tot)
	}
	if n := ActiveWeeks(tot); n != 2 {
		t.Fatalf("active weeks = %d, want 2", n)
	}
}

func TestBlockUnit(t *testing.T) {
	cases := []struct{ max, rows, want int }{
		{0, 17, 1}, {12, 17, 1}, {30, 17, 2}, {173, 17, 10}, {176, 17, 20}, {900, 17, 100},
	}
	for _, c := range cases {
		if got := BlockUnit(c.max, c.rows); got != c.want {
			t.Errorf("BlockUnit(%d, %d) = %d, want %d", c.max, c.rows, got, c.want)
		}
	}
}
