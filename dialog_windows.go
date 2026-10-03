package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func showError(title, msg string) {
	fmt.Fprintln(os.Stderr, title+": "+msg)

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getFileType := kernel32.NewProc("GetFileType")
	if ft, _, _ := getFileType.Call(os.Stdout.Fd()); ft != 0 {
		return
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	t, err1 := syscall.UTF16PtrFromString(title)
	m, err2 := syscall.UTF16PtrFromString(msg)
	if err1 != nil || err2 != nil {
		return
	}
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00000010)
}
