package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// unzipInto extracts a zip archive, preserving relative paths and refusing
// entries that would escape the destination directory.
func unzipInto(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	cleanDest := filepath.Clean(dest)
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		target := filepath.Join(cleanDest, name)
		if !within(cleanDest, target) {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := ensureDir(target); err != nil {
				return err
			}
			continue
		}
		if err := ensureDir(filepath.Dir(target)); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()|0o200)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

// extractTarGz extracts a .tar.gz archive (used for Linux JDKs), including
// symlinks and hard links.
func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(dest)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		target := filepath.Join(cleanDest, name)
		if !within(cleanDest, target) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := ensureDir(target); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := ensureDir(filepath.Dir(target)); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode()|0o200)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := ensureDir(filepath.Dir(target)); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				// Symlinks are not essential for a JDK to run; ignore failures.
				continue
			}
		case tar.TypeLink:
			if err := ensureDir(filepath.Dir(target)); err != nil {
				return err
			}
			_ = os.Remove(target)
			_ = os.Link(filepath.Join(cleanDest, filepath.Clean(hdr.Linkname)), target)
		}
	}
	return nil
}

// unzipFlat extracts native binaries (.so/.dll/.dylib) to the root of dest,
// discarding directory structure so a single java.library.path works.
func unzipFlat(archive, dest string) (int, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	if err := ensureDir(dest); err != nil {
		return 0, err
	}
	n := 0
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".so") && !strings.HasSuffix(lower, ".dll") &&
			!strings.HasSuffix(lower, ".dylib") && !strings.HasSuffix(lower, ".jnilib") {
			continue
		}
		target := filepath.Join(dest, name)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			continue
		}
		_, _ = io.Copy(out, rc)
		out.Close()
		rc.Close()
		n++
	}
	return n, nil
}

// within reports whether target is inside root.
func within(root, target string) bool {
	if target == root {
		return true
	}
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}

// findJavaHome locates the extracted JDK directory that contains bin/java.
func findJavaHome(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "jdk-") {
			continue
		}
		home := filepath.Join(root, e.Name())
		if fileExists(filepath.Join(home, "bin", exeName("java"))) {
			return home
		}
	}
	return ""
}

// runtimeDirForJava reports the target directory name for a JDK major version.
func runtimeDirForJava(root string, major int) string {
	return filepath.Join(root, fmt.Sprintf("java-%d", major))
}
