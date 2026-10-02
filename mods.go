package main

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// modProject is one entry of the curated mod set.
type modProject struct {
	ID   string
	Name string
}

// Default mods, using stable Modrinth project IDs (same set as launch.sh).
var defaultMods = []modProject{
	{"P7dR8mSH", "Fabric API"},
	{"AANobbMI", "Sodium"},
	{"YL57xqU", "Iris Shaders"},
	{"uXXizFIs", "FerriteCore"},
	{"gvQqBUqZ", "Lithium"},
	{"NNAgCjsB", "Entity Culling"},
	{"5ZwdcRci", "ImmediatelyFast"},
	{"PtjYWJkn", "Sodium Extra"},
	{"1IjD5062", "Continuity"},
	{"9s6osm5g", "Cloth Config"},
	{"mOgUt4GM", "Mod Menu"},
	{"w7ThoJFB", "Zoomify"},
	{"fQEb0iXm", "Krypton"},
	{"9eGKb6K1", "Simple Voice Chat"},
}

type modrinthVersion struct {
	VersionNumber string `json:"version_number"`
	Files         []struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
		Primary  bool   `json:"primary"`
		Size     int64  `json:"size"`
		Hashes   struct {
			SHA1 string `json:"sha1"`
		} `json:"hashes"`
	} `json:"files"`
	Dependencies []struct {
		ProjectID      string `json:"project_id"`
		DependencyType string `json:"dependency_type"`
	} `json:"dependencies"`
}

type modSummary struct {
	Name    string
	Version string
	File    string
}

// latestModVersion returns the newest Fabric build of a project for a version.
func (a *App) latestModVersion(ctx context.Context, projectID, mcVersion string) (*modrinthVersion, error) {
	u := "https://api.modrinth.com/v2/project/" + url.PathEscape(projectID) + "/version" +
		"?game_versions=" + url.QueryEscape(`["`+mcVersion+`"]`) +
		"&loaders=" + url.QueryEscape(`["fabric"]`)
	var versions []modrinthVersion
	if err := a.DL.GetJSON(ctx, u, &versions); err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("keine Fabric-Version fuer %s", mcVersion)
	}
	return &versions[0], nil
}

func pickPrimary(v *modrinthVersion) (downloadURL, filename, sha1 string, size int64, ok bool) {
	if len(v.Files) == 0 {
		return "", "", "", 0, false
	}
	f := v.Files[0]
	for _, cand := range v.Files {
		if cand.Primary {
			f = cand
			break
		}
	}
	return f.URL, f.Filename, f.Hashes.SHA1, f.Size, true
}

// collectMods resolves the curated mods plus their required dependencies.
func (a *App) collectMods(ctx context.Context, mcVersion string) (tasks []DownloadTask, summary []modSummary, managed []string, failures []string) {
	seen := map[string]bool{}
	queue := make([]string, 0, len(defaultMods))
	names := map[string]string{}
	for _, m := range defaultMods {
		queue = append(queue, m.ID)
		names[m.ID] = m.Name
	}

	guard := 0
	for len(queue) > 0 && guard < 64 {
		guard++
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			continue
		}
		seen[pid] = true

		v, err := a.latestModVersion(ctx, pid, mcVersion)
		if err != nil {
			failures = append(failures, fmt.Sprintf("Mod %s: %v", pid, err))
			continue
		}
		dl, filename, sha1, size, ok := pickPrimary(v)
		if !ok {
			continue
		}
		filename = sanitizeName(filename)
		tasks = append(tasks, DownloadTask{
			URL:  dl,
			Dest: filepath.Join(a.Paths.Mods, filename),
			SHA1: sha1,
			Size: size,
		})
		managed = append(managed, filename)
		if name, ok := names[pid]; ok {
			summary = append(summary, modSummary{Name: name, Version: v.VersionNumber, File: filename})
		}
		for _, dep := range v.Dependencies {
			if dep.DependencyType == "required" && dep.ProjectID != "" && !seen[dep.ProjectID] {
				queue = append(queue, dep.ProjectID)
			}
		}
	}
	return tasks, summary, managed, failures
}

// installMods downloads mods and removes files this launcher managed before.
func (a *App) installMods(ctx context.Context, mcVersion string) error {
	a.Log.Printf("[7/8] Mods (Modrinth-API)")
	tasks, summary, managed, resolveFail := a.collectMods(ctx, mcVersion)
	for _, f := range resolveFail {
		a.Log.Printf("    WARNUNG: %s", f)
	}

	failures := a.DL.Run(ctx, "Mods", tasks)
	for _, f := range failures {
		a.Log.Printf("    WARNUNG: %s", f)
	}

	// Remove mods we installed previously that are no longer wanted.
	if old, err := readLines(a.Paths.ManagedModsFile); err == nil {
		keep := map[string]bool{}
		for _, m := range managed {
			keep[m] = true
		}
		for _, name := range old {
			if name != "" && !keep[name] {
				_ = os.Remove(filepath.Join(a.Paths.Mods, name))
			}
		}
	}
	_ = writeLines(a.Paths.ManagedModsFile, managed)

	if len(summary) > 0 {
		a.Log.Printf("    Installierte Mods:")
		for _, m := range summary {
			flag := "ok "
			if !fileExists(filepath.Join(a.Paths.Mods, m.File)) {
				flag = "!! "
			}
			a.Log.Printf("      %s%-18s %s", flag, m.Name, m.Version)
		}
	}
	return nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, strings.TrimSpace(sc.Text()))
	}
	return out, sc.Err()
}

func writeLines(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
