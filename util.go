package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// sha1File returns the lowercase hex SHA-1 of a file.
func sha1File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// humanBytes formats a byte count for humans.
func humanBytes(b int64) string {
	const (
		gb = 1 << 30
		mb = 1 << 20
		kb = 1 << 10
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// classpathSep is the OS-specific java classpath separator.
func classpathSep() string {
	if runtime.GOOS == "windows" {
		return ";"
	}
	return ":"
}

// exeName appends .exe on Windows.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// osName maps the Go OS to the name Mojang uses in library rules.
func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}

// ensureDir creates a directory (and parents) if needed.
func ensureDir(p string) error {
	if p == "" {
		return nil
	}
	return os.MkdirAll(p, 0o755)
}

// sanitizeName makes a string safe to use as a file name on every OS.
func sanitizeName(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
}

// isNativePath reports whether a Maven path refers to a native library jar
// rather than a plain classpath jar.
func isNativePath(p string) bool {
	l := strings.ToLower(p)
	return strings.Contains(l, "natives-") || strings.Contains(l, "-natives")
}

// nativeArchOK filters arch-suffixed native jars. Modern version manifests
// list e.g. both `natives-windows` and `natives-windows-arm64` with the same
// OS rule, so the architecture must be selected from the artifact name.
//
// On Linux only a single `natives-linux` jar is published (no arm64 suffix),
// so it is accepted for every Linux architecture.
func nativeArchOK(path, goarch string) bool {
	p := strings.ToLower(path)
	hasArm64 := strings.Contains(p, "-arm64")
	if goarch == "arm64" {
		return hasArm64 || strings.Contains(p, "-natives-linux")
	}
	// 32-bit and 64-bit x86 share the un-suffixed natives.
	return !hasArm64
}
