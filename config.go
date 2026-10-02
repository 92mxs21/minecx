package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

const (
	appName = "minecx"
	version = "1.0.0"
)

// Config holds everything the user can influence from the command line.
type Config struct {
	Dir       string // install / game directory
	MCVersion string // "latest" or a specific id such as "1.21.4"
	User      string // offline-mode player name
	RamGB     int    // max heap in GB (0 = auto)
	Jobs      int    // parallel downloads (0 = auto)
	Update    bool   // ignore pinned versions and move to latest
	SetupOnly bool   // install but do not launch
	DryRun    bool   // print the launch command, do not launch
	Clean     bool   // delete the install directory and exit
	Verbose   bool   // verbose logging
	NoMods    bool   // skip the Modrinth mod installation
}

// State is the small JSON file used to pin versions between runs.
type State struct {
	MCVersion   string `json:"mc_version"`
	Loader      string `json:"loader"`
	JavaMajor   int    `json:"java_major"`
	AssetsIndex string `json:"assets_index"`
	SetupTime   string `json:"setup_time"`
}

// Paths are all directories used by a single installation.
type Paths struct {
	Root            string
	Versions        string
	Libraries       string
	Assets          string
	Natives         string
	Mods            string
	Logs            string
	Runtime         string
	Cache           string
	Config          string
	Shaderpacks     string
	StateFile       string
	ManagedModsFile string
}

func buildPaths(root string) Paths {
	return Paths{
		Root:            root,
		Versions:        filepath.Join(root, "versions"),
		Libraries:       filepath.Join(root, "libraries"),
		Assets:          filepath.Join(root, "assets"),
		Natives:         filepath.Join(root, "natives"),
		Mods:            filepath.Join(root, "mods"),
		Logs:            filepath.Join(root, "logs"),
		Runtime:         filepath.Join(root, "runtime"),
		Cache:           filepath.Join(root, ".cache"),
		Config:          filepath.Join(root, "config"),
		Shaderpacks:     filepath.Join(root, "shaderpacks"),
		StateFile:       filepath.Join(root, ".minecx-state.json"),
		ManagedModsFile: filepath.Join(root, ".managed-mods"),
	}
}

// Ensure creates every directory the launcher writes to.
func (p Paths) Ensure() error {
	for _, d := range []string{
		p.Versions, p.Libraries, p.Assets, p.Natives, p.Mods, p.Logs,
		p.Runtime, p.Cache, p.Config, p.Shaderpacks,
		filepath.Join(p.Natives, "jna"),
		filepath.Join(p.Natives, "lwjgl"),
		filepath.Join(p.Natives, "netty"),
	} {
		if err := ensureDir(d); err != nil {
			return err
		}
	}
	return nil
}

// defaultDataDir picks a user-writable persistent location.
//
// SteamOS has an immutable, read-only root filesystem, so the install must
// live under the user's home. On Linux we follow the XDG base directory spec;
// on Windows we use %APPDATA%.
func defaultDataDir() string {
	if runtime.GOOS == "windows" {
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, appName)
		}
		if up := os.Getenv("USERPROFILE"); up != "" {
			return filepath.Join(up, appName)
		}
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, appName)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", appName)
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, appName)
	}
	return filepath.Join(home, ".local", "share", appName)
}

func loadState(path string) State {
	var s State
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func saveState(path string, s State) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
