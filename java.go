package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
)

var javaVersionRE = regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`)

// javaMajor reads the major version from `java -version` output.
func javaMajor(path string) (int, error) {
	out, err := exec.Command(path, "-version").CombinedOutput()
	if err != nil && len(out) == 0 {
		return 0, err
	}
	m := javaVersionRE.FindSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("Java-Version nicht erkennbar")
	}
	major, _ := strconv.Atoi(string(m[1]))
	if major == 1 && len(m[2]) > 0 {
		major, _ = strconv.Atoi(string(m[2]))
	}
	return major, nil
}

// findJava returns a Java executable that is at least minMajor, or "".
func (a *App) findJava(minMajor int) (string, int) {
	var candidates []string
	if jh := os.Getenv("JAVA_HOME"); jh != "" {
		candidates = append(candidates, filepath.Join(jh, "bin", exeName("java")))
	}
	if p, err := exec.LookPath("java"); err == nil {
		candidates = append(candidates, p)
	}
	// A JDK we installed on an earlier run.
	candidates = append(candidates, filepath.Join(runtimeDirForJava(a.Paths.Runtime, minMajor), "bin", exeName("java")))

	for _, c := range candidates {
		if !fileExists(c) {
			continue
		}
		m, err := javaMajor(c)
		if err != nil {
			a.Log.Verbosef("Java-Kandidat %s nicht nutzbar: %v", c, err)
			continue
		}
		if m >= minMajor {
			return c, m
		}
		a.Log.Verbosef("Java-Kandidat %s hat Version %d (< %d)", c, m, minMajor)
	}
	return "", 0
}

func adoptiumArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	case "386":
		return "x86"
	case "arm":
		return "arm"
	default:
		return goarch
	}
}

// ensureJava finds or installs a suitable JDK and returns the java binary path.
func (a *App) ensureJava(ctx context.Context, required int) (string, error) {
	if bin, major := a.findJava(required); bin != "" {
		a.Log.Printf("    Java      : %s (Version %d)", bin, major)
		return bin, nil
	}

	a.Log.Printf("    Kein Java >= %d gefunden - installiere Temurin JDK %d", required, required)

	osPart := "linux"
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		osPart = "windows"
		ext = ".zip"
	}
	url := fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/%d/ga/%s/%s/jdk/hotspot/normal/eclipse",
		required, osPart, adoptiumArch(runtime.GOARCH))
	archive := filepath.Join(a.Paths.Cache, fmt.Sprintf("temurin-%d%s", required, ext))

	if err := ensureDir(a.Paths.Cache); err != nil {
		return "", err
	}
	failures := a.DL.Run(ctx, "Temurin", []DownloadTask{{URL: url, Dest: archive}})
	if len(failures) > 0 {
		return "", fmt.Errorf("JDK-Download fehlgeschlagen: %s", failures[0])
	}
	target := runtimeDirForJava(a.Paths.Runtime, required)
	if fileExists(filepath.Join(target, "bin", exeName("java"))) {
		return filepath.Join(target, "bin", exeName("java")), nil
	}
	if err := ensureDir(a.Paths.Runtime); err != nil {
		return "", err
	}

	var err error
	if runtime.GOOS == "windows" {
		err = unzipInto(archive, a.Paths.Runtime)
	} else {
		err = extractTarGz(archive, a.Paths.Runtime)
	}
	if err != nil {
		return "", fmt.Errorf("JDK entpacken fehlgeschlagen: %w", err)
	}
	found := findJavaHome(a.Paths.Runtime)
	if found == "" {
		return "", fmt.Errorf("im JDK-Archiv wurde kein bin/java gefunden")
	}
	if found != target {
		_ = os.RemoveAll(target)
		if err := os.Rename(found, target); err != nil {
			return "", err
		}
	}
	bin := filepath.Join(target, "bin", exeName("java"))
	if !fileExists(bin) {
		return "", fmt.Errorf("JDK-Installation unvollstaendig: %s", bin)
	}
	_ = os.Remove(archive)
	return bin, nil
}
