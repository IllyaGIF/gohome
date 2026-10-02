package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	Name        string   `json:"name"`
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Kind        string   `json:"kind"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Filter      []string `json:"filter,omitempty"`
}

type Project struct {
	Dir     string
	Config  Config
	Sources map[string]string
}

func Default(name, kind string) Config {
	c := Config{Name: name, ID: "com.gohome." + strings.ToLower(strings.ReplaceAll(name, "_", "-")), Version: "1.0.0", Kind: kind, Description: "Built with gohome", Author: "gohome"}
	if kind == "tweak" {
		c.Filter = []string{"com.apple.springboard"}
	}
	return c
}

func (c Config) Validate() error {
	if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`).MatchString(c.Name) {
		return fmt.Errorf("name must start with a letter and contain only letters, numbers, - or _ (max 64)")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+$`).MatchString(c.ID) {
		return fmt.Errorf("id must be a lowercase Debian package identifier")
	}
	if c.Kind != "tweak" && c.Kind != "app" {
		return fmt.Errorf("kind must be tweak or app")
	}
	if c.Kind == "app" && (!regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`).MatchString(c.ID)) {
		return fmt.Errorf("app id must be a reverse-domain bundle identifier")
	}
	if !regexp.MustCompile(`^[0-9][A-Za-z0-9.+~]*(?:-[A-Za-z0-9.+~]+)?$`).MatchString(c.Version) {
		return fmt.Errorf("invalid package version")
	}
	if c.Kind == "app" && !regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){0,2}$`).MatchString(c.Version) {
		return fmt.Errorf("app version must contain one to three numeric components, e.g. 1.0.0")
	}
	for _, text := range []string{c.Description, c.Author} {
		if strings.ContainsAny(text, "\r\n\x00") {
			return fmt.Errorf("description and author must be single-line text")
		}
	}
	if c.Kind == "tweak" && len(c.Filter) == 0 {
		return fmt.Errorf("tweak filter must contain at least one target bundle identifier")
	}
	for _, filter := range c.Filter {
		if filter == "" || strings.ContainsAny(filter, "\r\n\x00") {
			return fmt.Errorf("invalid bundle filter")
		}
	}
	return nil
}

func Load(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	stat, err := os.Stat(abs)
	if err != nil {
		return Project{}, err
	}
	dir := abs
	if !stat.IsDir() {
		dir = filepath.Dir(abs)
	}
	name := filepath.Base(dir)
	if !stat.IsDir() {
		name = strings.TrimSuffix(filepath.Base(abs), ".go")
	}
	cfg := Default(name, "tweak")
	data, err := os.ReadFile(filepath.Join(dir, "gohome.json"))
	if err == nil {
		cfg = Config{}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return Project{}, fmt.Errorf("gohome.json: %w", err)
		}
		if cfg.Version == "" {
			cfg.Version = "1.0.0"
		}
		if cfg.Author == "" {
			cfg.Author = "gohome"
		}
	} else if !os.IsNotExist(err) {
		return Project{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Project{}, err
	}
	paths := []string{abs}
	if stat.IsDir() {
		paths, err = goSources(abs)
		if err != nil {
			return Project{}, err
		}
	}
	sources := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return Project{}, err
		}
		sources[path] = string(data)
	}
	if len(sources) == 0 {
		return Project{}, fmt.Errorf("no .go files in %s", dir)
	}
	return Project{Dir: dir, Config: cfg, Sources: sources}, nil
}

// goSources walks a project directory so that views can live in their own
// files and subdirectories, skipping tests, build output and hidden trees.
func goSources(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path == root {
				return nil
			}
			if skipDir[name] || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

var skipDir = map[string]bool{"build": true, "vendor": true, "testdata": true, "bin": true}

func Create(path, kind, sdkSource string) error {
	name := filepath.Base(filepath.Clean(path))
	cfg := Default(name, kind)
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0755); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(path)
		}
	}()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	source := TweakExample
	if kind == "app" {
		source = AppExample
	}
	for name, content := range map[string]string{"gohome.json": string(data) + "\n", "main.go": source, ".gitignore": "build/\n"} {
		if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0644); err != nil {
			return err
		}
	}
	mod := fmt.Sprintf("module %s\n\ngo 1.22\n\nrequire gohome v0.0.0\n\nreplace gohome => ./.gohome/sdk\n", cfg.ID)
	if err := os.WriteFile(filepath.Join(path, "go.mod"), []byte(mod), 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(path, ".gohome/sdk/ios"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(path, ".gohome/sdk/go.mod"), []byte("module gohome\n\ngo 1.22\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(path, ".gohome/sdk/ios/ios.go"), []byte(sdkSource), 0644); err != nil {
		return err
	}
	complete = true
	return nil
}

const TweakExample = `package main

import "gohome/ios"

func main() {
	ios.Hook("SpringBoard", "applicationDidFinishLaunching:", func(self ios.Object, original func(ios.Object, ios.Object), application ios.Object) {
		original(self, application)
		ios.Alert("gohome", "Твик написан на Go")
	})
}
`

const AppExample = `package main

import "gohome/ios"

var taps int

func tapped() {
	taps++
	ios.Alert("gohome", "Нажатий: " + ios.Text(taps))
}

func main() {
	ios.Label("Привет из Go", 20, 60, 280, 40)
	ios.Button("Нажми меня", 20, 120, 280, 44, tapped)
}
`
