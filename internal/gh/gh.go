// Package gh reads the public data the page is drawn from.
package gh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/mordilloSan/mordilloSan/internal/profile"
)

// Weeks is how much history the page shows.
const Weeks = 52

// Client talks to the GitHub API.
type Client struct {
	token string
	base  string
	http  *http.Client
	// statsWait is the pause between retries while GitHub computes statistics.
	statsWait time.Duration
}

// New returns a client that authenticates with token.
func New(token string) *Client {
	return &Client{
		token:     token,
		base:      "https://api.github.com",
		http:      &http.Client{Timeout: 30 * time.Second},
		statsWait: 3 * time.Second,
	}
}

var errAccepted = errors.New("statistics are still being computed")

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	switch {
	case res.StatusCode == http.StatusAccepted:
		return res.Header, errAccepted
	case res.StatusCode == http.StatusNotFound, res.StatusCode == http.StatusConflict:
		return res.Header, errNone
	case res.StatusCode >= 300:
		return nil, fmt.Errorf("%s %s: %s: %.200s", method, path, res.Status, data)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("%s %s: %w", method, path, err)
		}
	}
	return res.Header, nil
}

// errNone means the thing asked for does not exist (no release, empty repo).
var errNone = errors.New("not found")

// Fetch collects everything the page shows about login.
func (c *Client) Fetch(ctx context.Context, login string) (profile.Profile, error) {
	p := profile.Profile{Login: login, Now: time.Now().UTC()}

	var user struct {
		Name string `json:"name"`
	}
	if _, err := c.do(ctx, http.MethodGet, "/users/"+login, nil, &user); err != nil {
		return p, err
	}
	p.Name = user.Name
	if p.Name == "" {
		p.Name = login
	}

	total, err := c.contributions(ctx, login)
	if err != nil {
		return p, err
	}
	p.Contributions = total

	for page := 1; ; page++ {
		var repos []struct {
			Name        string    `json:"name"`
			FullName    string    `json:"full_name"`
			Description string    `json:"description"`
			Language    string    `json:"language"`
			HTMLURL     string    `json:"html_url"`
			Stars       int       `json:"stargazers_count"`
			Fork        bool      `json:"fork"`
			Archived    bool      `json:"archived"`
			PushedAt    time.Time `json:"pushed_at"`
		}
		path := fmt.Sprintf("/users/%s/repos?type=owner&per_page=100&page=%d", login, page)
		if _, err := c.do(ctx, http.MethodGet, path, nil, &repos); err != nil {
			return p, err
		}
		for _, r := range repos {
			// The profile repository is this page itself, not a project.
			if r.Fork || r.Name == login {
				continue
			}
			repo := profile.Repo{
				Name: r.Name, Description: r.Description, Language: r.Language, URL: r.HTMLURL,
				Stars: r.Stars, Archived: r.Archived, PushedAt: r.PushedAt,
			}
			if err := c.fill(ctx, r.FullName, &repo); err != nil {
				return p, err
			}
			p.Repos = append(p.Repos, repo)
		}
		if len(repos) < 100 {
			break
		}
	}
	return p, nil
}

// fill adds the per-repository figures: latest release, commit count and
// weekly activity.
func (c *Client) fill(ctx context.Context, full string, r *profile.Repo) error {
	var rel struct {
		Tag         string    `json:"tag_name"`
		PublishedAt time.Time `json:"published_at"`
	}
	switch _, err := c.do(ctx, http.MethodGet, "/repos/"+full+"/releases/latest", nil, &rel); {
	case err == nil:
		r.ReleaseTag, r.ReleasedAt = rel.Tag, rel.PublishedAt
	case !errors.Is(err, errNone):
		return err
	}

	var one []json.RawMessage
	switch h, err := c.do(ctx, http.MethodGet, "/repos/"+full+"/commits?per_page=1", nil, &one); {
	case err == nil:
		r.Commits = lastPage(h.Get("Link"), len(one))
	case !errors.Is(err, errNone):
		return err
	}

	weekly, err := c.participation(ctx, full)
	if err != nil {
		return err
	}
	r.Weekly = weekly
	return nil
}

var lastRE = regexp.MustCompile(`[?&]page=(\d+)>; rel="last"`)

// lastPage reads the total from a one-item-per-page Link header.
func lastPage(link string, fallback int) int {
	if m := lastRE.FindStringSubmatch(link); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return fallback
}

// participation returns the owner's commits per week for the last year.
// GitHub computes these on demand and answers 202 until they are ready.
func (c *Client) participation(ctx context.Context, full string) ([]int, error) {
	var out struct {
		Owner []int `json:"owner"`
	}
	for try := 0; ; try++ {
		_, err := c.do(ctx, http.MethodGet, "/repos/"+full+"/stats/participation", nil, &out)
		if err == nil {
			break
		}
		if errors.Is(err, errNone) {
			return make([]int, Weeks), nil
		}
		if !errors.Is(err, errAccepted) || try >= 10 {
			return nil, fmt.Errorf("weekly activity for %s: %w", full, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.statsWait):
		}
	}
	w := out.Owner
	if len(w) > Weeks {
		w = w[len(w)-Weeks:]
	}
	for len(w) < Weeks {
		w = append([]int{0}, w...)
	}
	return w, nil
}

// contributions is the number GitHub shows above the contribution calendar.
func (c *Client) contributions(ctx context.Context, login string) (int, error) {
	query := `query($login:String!){user(login:$login){contributionsCollection{contributionCalendar{totalContributions}}}}`
	body, err := json.Marshal(map[string]any{"query": query, "variables": map[string]string{"login": login}})
	if err != nil {
		return 0, err
	}
	var out struct {
		Data struct {
			User *struct {
				ContributionsCollection struct {
					ContributionCalendar struct {
						TotalContributions int `json:"totalContributions"`
					} `json:"contributionCalendar"`
				} `json:"contributionsCollection"`
			} `json:"user"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if _, err := c.do(ctx, http.MethodPost, "/graphql", bytes.NewReader(body), &out); err != nil {
		return 0, err
	}
	if len(out.Errors) > 0 {
		return 0, fmt.Errorf("contributions: %s", out.Errors[0].Message)
	}
	if out.Data.User == nil {
		return 0, fmt.Errorf("contributions: user %q not found", login)
	}
	return out.Data.User.ContributionsCollection.ContributionCalendar.TotalContributions, nil
}
