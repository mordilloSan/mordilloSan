// Package profile holds the data behind the profile page and the rules that
// decide which repository goes where on it.
package profile

import (
	"math"
	"sort"
	"time"
)

// Repo is one public, non-fork repository.
type Repo struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	URL         string    `json:"url"`
	Stars       int       `json:"stars"`
	Commits     int       `json:"commits"`
	Archived    bool      `json:"archived"`
	ReleaseTag  string    `json:"releaseTag"`
	ReleasedAt  time.Time `json:"releasedAt"`
	PushedAt    time.Time `json:"pushedAt"`
	// Weekly is the owner's commits per week, oldest first, ending this week.
	Weekly []int `json:"weekly"`
}

// YearCommits is the number of commits in the weeks covered by Weekly.
func (r Repo) YearCommits() int {
	n := 0
	for _, w := range r.Weekly {
		n += w
	}
	return n
}

// Profile is everything the page shows.
type Profile struct {
	Login         string    `json:"login"`
	Name          string    `json:"name"`
	Contributions int       `json:"contributions"`
	Repos         []Repo    `json:"repos"`
	Now           time.Time `json:"now"`
}

// Layout says which repository gets which slot.
type Layout struct {
	Feature *Repo  // the large card, nil when there is nothing to feature
	Cards   []Repo // the small cards
	Table   []Repo // every repository, in display order
}

// Select applies the page rules:
//   - the feature card is the named repository, or else the one with the most
//     commits in the last year; archived repositories are never featured;
//   - the small cards are the other active repositories with the newest
//     releases, filled up with the most recently pushed ones if too few have
//     a release;
//   - the table lists every repository, newest release first, archived last.
func Select(repos []Repo, feature string, maxCards int) Layout {
	var l Layout
	active := make([]Repo, 0, len(repos))
	var archived []Repo
	for _, r := range repos {
		if r.Archived {
			archived = append(archived, r)
		} else {
			active = append(active, r)
		}
	}
	byRelease(active)
	byRelease(archived)
	l.Table = append(append(l.Table, active...), archived...)

	for i := range active {
		if feature != "" && active[i].Name == feature {
			l.Feature = &active[i]
			break
		}
	}
	if l.Feature == nil {
		for i := range active {
			if l.Feature == nil || busier(active[i], *l.Feature) {
				l.Feature = &active[i]
			}
		}
	}
	for _, r := range active {
		if len(l.Cards) >= maxCards {
			break
		}
		if l.Feature != nil && r.Name == l.Feature.Name {
			continue
		}
		l.Cards = append(l.Cards, r)
	}
	return l
}

func busier(a, b Repo) bool {
	if ay, by := a.YearCommits(), b.YearCommits(); ay != by {
		return ay > by
	}
	if a.Commits != b.Commits {
		return a.Commits > b.Commits
	}
	return a.Name < b.Name
}

// byRelease sorts released repositories first, newest release first, then the
// unreleased ones by most recent push.
func byRelease(rs []Repo) {
	sort.SliceStable(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		ar, br := a.ReleaseTag != "", b.ReleaseTag != ""
		if ar != br {
			return ar
		}
		at, bt := a.PushedAt, b.PushedAt
		if ar {
			at, bt = a.ReleasedAt, b.ReleasedAt
		}
		if !at.Equal(bt) {
			return at.After(bt)
		}
		return a.Name < b.Name
	})
}

// WeeklyTotals sums the weekly commits of all repositories. Repositories with
// a shorter history are aligned to the most recent week.
func WeeklyTotals(repos []Repo, weeks int) []int {
	tot := make([]int, weeks)
	for _, r := range repos {
		off := weeks - len(r.Weekly)
		for i, w := range r.Weekly {
			if k := i + off; k >= 0 {
				tot[k] += w
			}
		}
	}
	return tot
}

// ActiveWeeks counts the weeks with at least one commit.
func ActiveWeeks(weekly []int) int {
	n := 0
	for _, w := range weekly {
		if w > 0 {
			n++
		}
	}
	return n
}

// BlockUnit picks how many commits one block of the year graph stands for:
// the smallest round number that keeps the tallest week within rows blocks.
func BlockUnit(peak, rows int) int {
	for mag := 1; ; mag *= 10 {
		for _, m := range []int{1, 2, 5} {
			if u := m * mag; Blocks(peak, u) <= rows {
				return u
			}
		}
	}
}

// Blocks is the number of blocks drawn for n commits: rounded to the unit,
// but never zero for a week that had any commits.
func Blocks(n, unit int) int {
	if n <= 0 {
		return 0
	}
	return max(1, int(math.Round(float64(n)/float64(unit))))
}
