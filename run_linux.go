package main

import (
	"os"
	"syscall"
)

// runGame replaces the launcher process with the game on Linux. This forwards
// signals (Ctrl+C) directly to Java and leaves no parent process behind.
func runGame(bin string, args []string, dir string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	argv := append([]string{bin}, args...)
	return syscall.Exec(bin, argv, os.Environ())
}
