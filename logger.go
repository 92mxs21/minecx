package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Logger writes to both the terminal and a log file. It also owns the simple
// single-line progress display used while downloading.
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	verbose bool
	lastLen int
}

func NewLogger(logPath string, verbose bool) *Logger {
	l := &Logger{verbose: verbose}
	if logPath != "" {
		_ = ensureDir(filepath.Dir(logPath))
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err == nil {
			l.file = f
		}
	}
	return l
}

func (l *Logger) Close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}

func (l *Logger) write(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Print(s)
	if l.file != nil {
		_, _ = l.file.WriteString(s)
	}
}

func (l *Logger) clearProgressLocked() {
	if l.lastLen > 0 {
		fmt.Printf("\r%s\r", strings.Repeat(" ", l.lastLen+2))
		l.lastLen = 0
	}
}

func (l *Logger) Printf(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearProgressLocked()
	l.write(fmt.Sprintf("%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...)))
}

func (l *Logger) Verbosef(format string, a ...any) {
	if !l.verbose {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearProgressLocked()
	l.write(fmt.Sprintf("%s  debug: %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...)))
}

// Progress paints a live status line. Safe to call from one goroutine.
func (l *Logger) Progress(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(s) < l.lastLen {
		s += strings.Repeat(" ", l.lastLen-len(s))
	}
	fmt.Printf("\r  %s", s)
	if len(s) > l.lastLen {
		l.lastLen = len(s)
	}
}

func (l *Logger) ProgressEnd() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clearProgressLocked()
}
