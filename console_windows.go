package main

import (
	"os"
	"syscall"
)

func attachConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getFileType := kernel32.NewProc("GetFileType")
	ft, _, _ := getFileType.Call(os.Stdout.Fd())
	if ft != 0 {
		return
	}
	attach := kernel32.NewProc("AttachConsole")
	_, _, _ = attach.Call(uintptr(^uint32(0)))
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = f
	}
}
