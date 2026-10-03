package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type fileConfig struct {
	Path  string
	Name  string
	Skin  string
	Model string
	MC    string
	Ram   int
	Jobs  int
}

func findConfigFile(explicit, installDir string) string {
	if explicit != "" {
		if fileExists(explicit) {
			return explicit
		}
		return ""
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "config.txt"))
	}
	candidates = append(candidates, filepath.Join(installDir, "config.txt"))
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "config.txt"))
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func parseConfigFile(path string) (fileConfig, error) {
	fc := fileConfig{Path: path}
	f, err := os.Open(path)
	if err != nil {
		return fc, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		key, val, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "name", "user", "username", "nick":
			fc.Name = val
		case "skin", "skinurl", "skinfile":
			fc.Skin = val
		case "model", "skinmodel", "slim":
			fc.Model = val
		case "mc", "version":
			fc.MC = val
		case "ram", "ramgb":
			if n, err := strconv.Atoi(val); err == nil {
				fc.Ram = n
			}
		case "jobs":
			if n, err := strconv.Atoi(val); err == nil {
				fc.Jobs = n
			}
		}
	}
	return fc, sc.Err()
}

const configTemplate = `# minecx config - set your name, then start again
name:
# skin: C:\path\to\skin.png   (or https://...)
model: steve
# ram: 6
mc: latest
`

func ensureConfigFile(installDir string) string {
	if existing := findConfigFile("", installDir); existing != "" {
		return existing
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "config.txt"))
	}
	candidates = append(candidates, filepath.Join(installDir, "config.txt"))
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "config.txt"))
	}
	for _, p := range candidates {
		if err := os.WriteFile(p, []byte(configTemplate), 0o644); err == nil {
			return p
		}
	}
	return ""
}

func splitKeyValue(line string) (string, string, bool) {
	if i := strings.IndexByte(line, ':'); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
	}
	if i := strings.IndexByte(line, '='); i >= 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
	}
	return "", "", false
}
