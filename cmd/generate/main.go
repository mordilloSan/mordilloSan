// Command generate draws the profile page: it reads the account's public
// data from GitHub, renders the images under assets/ and writes README.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mordilloSan/mordilloSan/internal/gh"
	"github.com/mordilloSan/mordilloSan/internal/profile"
	"github.com/mordilloSan/mordilloSan/internal/render"
)

// config is what config.json can set.
type config struct {
	User        string `json:"user"`
	Name        string `json:"name"` // shown as the headline; empty uses the GitHub profile name
	Tagline     string `json:"tagline"`
	Accent      string `json:"accent"`
	AccentLight string `json:"accentLight"`
	Feature     string `json:"feature"` // repository to feature; empty picks the most active
	Cards       int    `json:"cards"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := flag.String("config", "config.json", "settings file")
	out := flag.String("out", ".", "directory to write README.md and assets/ into")
	dataIn := flag.String("data", "", "draw from this saved data file instead of asking GitHub")
	dataOut := flag.String("save-data", "", "also save the fetched data to this file")
	sheet := flag.String("sheet", "", "also write whole-page preview images with this file prefix")
	flag.Parse()

	cfg := config{Accent: "#ff8a3d", AccentLight: "#c2540a", Cards: 4}
	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("%s: %w", *cfgPath, err)
	}
	if cfg.User == "" {
		cfg.User = os.Getenv("GITHUB_REPOSITORY_OWNER")
	}
	if cfg.User == "" {
		return errors.New(`set "user" in the config file`)
	}

	var p profile.Profile
	if *dataIn != "" {
		raw, err := os.ReadFile(*dataIn)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("%s: %w", *dataIn, err)
		}
	} else {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			token = os.Getenv("GH_TOKEN")
		}
		if token == "" {
			return errors.New("set GITHUB_TOKEN (or GH_TOKEN) so GitHub can be asked for the data")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if p, err = gh.New(token).Fetch(ctx, cfg.User); err != nil {
			return err
		}
	}
	if *dataOut != "" {
		raw, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*dataOut, raw, 0o644); err != nil {
			return err
		}
	}

	if cfg.Name != "" {
		p.Name = cfg.Name
	}
	layout := profile.Select(p.Repos, cfg.Feature, cfg.Cards)
	page := render.Build(p, layout, cfg.Tagline)
	theme := render.Theme{Accent: cfg.Accent, AccentLight: cfg.AccentLight}

	// Replace the generated images wholesale so a repository that drops off
	// the page does not leave its old image behind.
	assets := filepath.Join(*out, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		return err
	}
	old, err := filepath.Glob(filepath.Join(assets, "*.svg"))
	if err != nil {
		return err
	}
	for _, f := range old {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	for _, piece := range page.All() {
		if err := os.WriteFile(filepath.Join(assets, piece.FileName(theme)), []byte(piece.SVG(theme, render.ModeAuto)), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(*out, "README.md"), []byte(page.Readme("assets", theme)), 0o644); err != nil {
		return err
	}

	if *sheet != "" {
		for mode, name := range map[render.Mode]string{render.ModeDark: "dark", render.ModeLight: "light"} {
			if err := os.WriteFile(*sheet+"-"+name+".svg", []byte(page.Sheet(theme, mode)), 0o644); err != nil {
				return err
			}
		}
	}

	names := make([]string, len(layout.Cards))
	for i, r := range layout.Cards {
		names[i] = r.Name
	}
	feature := "none"
	if layout.Feature != nil {
		feature = layout.Feature.Name
	}
	fmt.Printf("%s: %d repos, feature %s, cards %s\n", cfg.User, len(p.Repos), feature, strings.Join(names, ", "))
	return nil
}
