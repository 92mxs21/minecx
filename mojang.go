package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ---------------------------------------------------------------------------
// Mojang / Piston data model
// ---------------------------------------------------------------------------

type VersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"versions"`
}

type Rule struct {
	Action string `json:"action"`
	OS     struct {
		Name string `json:"name"`
		Arch string `json:"arch"`
	} `json:"os"`
	Features map[string]bool `json:"features"`
}

type Artifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

type Library struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact    *Artifact           `json:"artifact"`
		Classifiers map[string]Artifact `json:"classifiers"`
	} `json:"downloads"`
	Natives map[string]string `json:"natives"`
	Rules   []Rule            `json:"rules"`
}

type VersionJSON struct {
	ID          string `json:"id"`
	MainClass   string `json:"mainClass"`
	JavaVersion struct {
		MajorVersion int `json:"majorVersion"`
	} `json:"javaVersion"`
	AssetIndex struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
	} `json:"assetIndex"`
	Downloads struct {
		Client struct {
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
			Path string `json:"path"`
		} `json:"client"`
	} `json:"downloads"`
	Logging struct {
		Client struct {
			File struct {
				ID   string `json:"id"`
				URL  string `json:"url"`
				SHA1 string `json:"sha1"`
			} `json:"file"`
		} `json:"client"`
	} `json:"logging"`
	Libraries []Library `json:"libraries"`
}

type AssetIndex struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

// MCResolved is everything needed to install one Minecraft version.
type MCResolved struct {
	ID         string
	Version    VersionJSON
	JavaMajor  int
	AssetID    string
	AssetURL   string
	AssetSHA1  string
	ClientURL  string
	ClientSHA1 string
	ClientSize int64
	ClientPath string
	Log4jURL   string
	Log4jID    string
	Log4jSHA1  string
}

// ResolvedLib is a pending download relative to a base directory.
type ResolvedLib struct {
	URL     string
	RelPath string
	SHA1    string
	Size    int64
}

// ---------------------------------------------------------------------------
// Rule evaluation (same algorithm as the official launcher)
// ---------------------------------------------------------------------------

func archMatches(want, goarch string) bool {
	switch want {
	case "x86":
		return goarch == "386"
	case "x86_64", "amd64":
		return goarch == "amd64"
	case "arm64", "aarch64":
		return goarch == "arm64"
	case "arm", "arm32":
		return goarch == "arm"
	}
	return false
}

func ruleMatches(r Rule, name, arch string) bool {
	if r.OS.Name != "" && r.OS.Name != name {
		return false
	}
	if r.OS.Arch != "" && !archMatches(r.OS.Arch, arch) {
		return false
	}
	// Demo mode and custom resolutions are never enabled by this launcher,
	// so any rule that depends on a feature does not apply.
	for _, v := range r.Features {
		if v {
			return false
		}
	}
	return true
}

// rulesAllow applies Mojang's "last matching rule wins" semantics.
func rulesAllow(rules []Rule, name, arch string) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, r := range rules {
		if ruleMatches(r, name, arch) {
			allowed = r.Action == "allow"
		}
	}
	return allowed
}

// ---------------------------------------------------------------------------
// Resolution
// ---------------------------------------------------------------------------

const pistonManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

func (a *App) resolveMinecraft(ctx context.Context) (*MCResolved, error) {
	var manifest VersionManifest
	if err := a.DL.GetJSON(ctx, pistonManifestURL, &manifest); err != nil {
		return nil, fmt.Errorf("Version-Manifest nicht ladbar: %w", err)
	}

	state := loadState(a.Paths.StateFile)
	wanted := a.Cfg.MCVersion
	if wanted == "" || strings.EqualFold(wanted, "latest") {
		newest := manifest.Latest.Release
		if !a.Cfg.Update && state.MCVersion != "" && state.MCVersion != newest {
			a.Log.Printf("Hinweis: neuere Minecraft-Version %s verfuegbar (aktiv: %s) - '--update' wechselt", newest, state.MCVersion)
			wanted = state.MCVersion
		} else {
			wanted = newest
		}
	}

	var metaURL string
	for _, v := range manifest.Versions {
		if v.ID == wanted {
			metaURL = v.URL
			break
		}
	}
	if metaURL == "" {
		return nil, fmt.Errorf("Minecraft-Version %q existiert nicht", wanted)
	}

	var vj VersionJSON
	if err := a.DL.GetJSON(ctx, metaURL, &vj); err != nil {
		return nil, fmt.Errorf("Versions-Metadaten nicht ladbar: %w", err)
	}

	javaMajor := vj.JavaVersion.MajorVersion
	if javaMajor == 0 {
		javaMajor = 21
	}

	clientPath := vj.Downloads.Client.Path
	if clientPath == "" {
		clientPath = filepath.Join("net", "minecraft", "client", wanted, "client-"+wanted+".jar")
	}

	return &MCResolved{
		ID:         wanted,
		Version:    vj,
		JavaMajor:  javaMajor,
		AssetID:    vj.AssetIndex.ID,
		AssetURL:   vj.AssetIndex.URL,
		AssetSHA1:  vj.AssetIndex.SHA1,
		ClientURL:  vj.Downloads.Client.URL,
		ClientSHA1: vj.Downloads.Client.SHA1,
		ClientSize: vj.Downloads.Client.Size,
		ClientPath: clientPath,
		Log4jURL:   vj.Logging.Client.File.URL,
		Log4jID:    vj.Logging.Client.File.ID,
		Log4jSHA1:  vj.Logging.Client.File.SHA1,
	}, nil
}

// collectVanilla splits the vanilla libraries into classpath jars and natives.
func collectVanilla(v VersionJSON) (libs, natives []ResolvedLib) {
	name := osName()
	arch := runtime.GOARCH
	seenNative := map[string]bool{}

	for i := range v.Libraries {
		l := &v.Libraries[i]
		if !rulesAllow(l.Rules, name, arch) {
			continue
		}
		if l.Downloads.Artifact != nil && l.Downloads.Artifact.URL != "" {
			a := l.Downloads.Artifact
			rl := ResolvedLib{URL: a.URL, RelPath: a.Path, SHA1: a.SHA1, Size: a.Size}
			if isNativePath(a.Path) {
				if nativeArchOK(a.Path, arch) && !seenNative[a.Path] {
					seenNative[a.Path] = true
					natives = append(natives, rl)
				}
			} else {
				libs = append(libs, rl)
			}
		}
		// Legacy libraries declare natives through a classifier map.
		if l.Natives != nil {
			if key, ok := l.Natives[name]; ok {
				if a, ok := l.Downloads.Classifiers[key]; ok && a.URL != "" {
					if nativeArchOK(a.Path, arch) && !seenNative[a.Path] {
						seenNative[a.Path] = true
						natives = append(natives, ResolvedLib{URL: a.URL, RelPath: a.Path, SHA1: a.SHA1, Size: a.Size})
					}
				}
			}
		}
	}
	return libs, natives
}

// collectAssets turns an asset index into a flat download list.
func collectAssets(index AssetIndex) []ResolvedLib {
	out := make([]ResolvedLib, 0, len(index.Objects))
	for _, obj := range index.Objects {
		if len(obj.Hash) < 2 {
			continue
		}
		head := obj.Hash[:2]
		out = append(out, ResolvedLib{
			URL:     "https://resources.download.minecraft.net/" + head + "/" + obj.Hash,
			RelPath: filepath.Join("objects", head, obj.Hash),
			SHA1:    obj.Hash,
			Size:    obj.Size,
		})
	}
	return out
}
