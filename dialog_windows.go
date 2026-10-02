package main

import (
	"syscall"
	"unsafe"
)

func showError(title, msg string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	t, err1 := syscall.UTF16PtrFromString(title)
	m, err2 := syscall.UTF16PtrFromString(msg)
	if err1 != nil || err2 != nil {
		return
	}
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00000010)
}
