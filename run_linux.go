package main

import (
	"os"
	"syscall"
)

func runGame(bin string, args []string, dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	argv := append([]string{bin}, args...)
	return syscall.Exec(bin, argv, os.Environ())
}
