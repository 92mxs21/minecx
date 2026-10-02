//go:build !windows && !linux

package main

// totalRAMGB returns 0 on platforms without a dedicated implementation.
func totalRAMGB() int { return 0 }
