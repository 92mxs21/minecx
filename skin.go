package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) setupSkin(ctx context.Context) error {
	src := strings.TrimSpace(a.Cfg.Skin)
	if src == "" {
		return nil
	}
	skinDir := filepath.Join(a.Paths.Config, "offlineskins")
	if err := ensureDir(skinDir); err != nil {
		return err
	}

	name := sanitizeName(a.Cfg.User)
	if name == "" {
		name = "player"
	}
	dest := filepath.Join(skinDir, name+".png")

	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		if err := a.downloadFile(ctx, src, dest); err != nil {
			a.Log.Printf("    WARNUNG: Skin-Download fehlgeschlagen: %v", err)
			return nil
		}
	} else {
		p := src
		if !filepath.IsAbs(p) {
			base := filepath.Dir(a.Cfg.ConfigFile)
			if a.Cfg.ConfigFile == "" {
				base, _ = os.Getwd()
			}
			p = filepath.Join(base, p)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			a.Log.Printf("    WARNUNG: Skin-Datei nicht gefunden: %s", p)
			return nil
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}

	model := strings.ToLower(strings.TrimSpace(a.Cfg.SkinModel))
	if model != "alex" {
		model = "steve"
	}
	cfg := fmt.Sprintf("{\n  \"selectedSkinName\": \"%s\",\n  \"defaultModel\": \"%s\"\n}\n", name, model)
	if err := os.WriteFile(filepath.Join(skinDir, "config.json"), []byte(cfg), 0o644); err != nil {
		return err
	}
	a.Log.Printf("    Skin      : %s (%s) -> %s", src, model, dest)
	return nil
}

func (a *App) downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := a.DL.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	if err := ensureDir(filepath.Dir(dest)); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
