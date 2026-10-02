package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

const fabricMaven = "https://maven.fabricmc.net/"

type FabricLoaderEntry struct {
	Loader struct {
		Version string `json:"version"`
	} `json:"loader"`
}

type FabricProfile struct {
	ID        string `json:"id"`
	MainClass string `json:"mainClass"`
	Libraries []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
	} `json:"libraries"`
}

// FabricResolved is the result of resolving the Fabric loader for a version.
type FabricResolved struct {
	Loader    string
	MainClass string
	ProfileID string
	Libraries []ResolvedLib
}

// mavenToURL converts a Maven coordinate into a URL and a relative path.
// Supports the optional classifier form group:artifact:version:classifier.
func mavenToURL(name, base string) (downloadURL, relPath string, ok bool) {
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return "", "", false
	}
	group := strings.ReplaceAll(parts[0], ".", "/")
	artifact, ver := parts[1], parts[2]
	classifier := ""
	if len(parts) > 3 {
		classifier = parts[3]
	}
	file := artifact + "-" + ver
	if classifier != "" {
		file += "-" + classifier
	}
	file += ".jar"
	relPath = group + "/" + artifact + "/" + ver + "/" + file
	if base == "" {
		base = fabricMaven
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + relPath, relPath, true
}

// resolveFabric finds the loader profile for a Minecraft version and pins it
// through the state file unless --update was given.
func (a *App) resolveFabric(ctx context.Context, mc *MCResolved) (*FabricResolved, error) {
	state := loadState(a.Paths.StateFile)

	var entries []FabricLoaderEntry
	loadersURL := "https://meta.fabricmc.net/v2/versions/loader/" + url.PathEscape(mc.ID)
	if err := a.DL.GetJSON(ctx, loadersURL, &entries); err != nil {
		return nil, fmt.Errorf("Fabric-Meta nicht erreichbar: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("Fabric hat fuer Minecraft %s noch keinen Loader", mc.ID)
	}
	newest := entries[0].Loader.Version

	loader := newest
	if !a.Cfg.Update && state.Loader != "" && state.Loader != newest {
		a.Log.Printf("Hinweis: neuerer Fabric-Loader %s verfuegbar (aktiv: %s) - '--update' wechselt", newest, state.Loader)
		loader = state.Loader
	}

	var profile FabricProfile
	profileURL := fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s/profile/json",
		url.PathEscape(mc.ID), url.PathEscape(loader))
	if err := a.DL.GetJSON(ctx, profileURL, &profile); err != nil {
		return nil, fmt.Errorf("Fabric-Profil %s nicht ladbar: %w", loader, err)
	}

	res := &FabricResolved{
		Loader:    loader,
		MainClass: profile.MainClass,
		ProfileID: profile.ID,
	}
	if res.MainClass == "" {
		res.MainClass = mc.Version.MainClass
	}
	for _, lib := range profile.Libraries {
		u, rel, ok := mavenToURL(lib.Name, lib.URL)
		if !ok {
			continue
		}
		res.Libraries = append(res.Libraries, ResolvedLib{
			URL:     u,
			RelPath: rel,
			SHA1:    lib.SHA1,
			Size:    lib.Size,
		})
	}
	return res, nil
}
