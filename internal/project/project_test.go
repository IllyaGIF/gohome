package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCreateAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hello")
	if err := Create(path, "tweak", "package ios\n"); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.Kind != "tweak" || len(p.Sources) != 1 || len(p.Config.Filter) != 1 {
		t.Fatal(p.Config)
	}
	if err := Create(path, "app", "package ios\n"); err == nil {
		t.Fatal("overwrote existing project")
	}
	if _, err := os.Stat(filepath.Join(path, ".gohome/sdk/ios/ios.go")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadWalksViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tabs")
	if err := Create(path, "app", "package ios\n"); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"views/home.go":       "package main\nfunc viewHome() {}\n",
		"views/settings.go":   "package main\nfunc viewSettings() {}\n",
		"views/deep/extra.go": "package main\nfunc viewExtra() {}\n",
		"views/home_test.go":  "package main\n",
		"build/skip.go":       "package main\n",
		".hidden/skip.go":     "package main\n",
		"vendor/skip.go":      "package main\n",
		"README.md":           "ignored\n",
	}
	for name, content := range files {
		full := filepath.Join(path, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"main.go", "views/home.go", "views/settings.go", "views/deep/extra.go"}
	if len(p.Sources) != len(want) {
		t.Fatalf("loaded %d sources, want %d: %v", len(p.Sources), len(want), sorted(p.Sources))
	}
	for _, name := range want {
		if _, ok := p.Sources[filepath.Join(path, name)]; !ok {
			t.Errorf("missing %s in %v", name, sorted(p.Sources))
		}
	}
}

func sorted(sources map[string]string) []string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, strings.TrimPrefix(name, "/"))
	}
	sort.Strings(names)
	return names
}

func TestInvalidMetadata(t *testing.T) {
	for _, change := range []func(*Config){
		func(c *Config) { c.Name = "../../evil" },
		func(c *Config) { c.ID = "com.test\nDepends: bad" },
		func(c *Config) { c.Version = "1-" },
		func(c *Config) { c.Filter = nil },
		func(c *Config) { c.Author = "someone\nArchitecture: all" },
		func(c *Config) { c.Kind = "app"; c.Version = "1.0-beta" },
	} {
		c := Default("Hello", "tweak")
		change(&c)
		if err := c.Validate(); err == nil {
			t.Fatal("accepted invalid metadata", c)
		}
	}
}
