package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Println("readdir:", err)
		os.Exit(1)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		path := filepath.Join(root, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			fmt.Println("parse skip", path, err)
			continue
		}
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, file); err != nil {
			fmt.Println("format skip", path, err)
			continue
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			fmt.Println("write fail", path, err)
			continue
		}
		fmt.Println("stripped", e.Name())
	}
}
