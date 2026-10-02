package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const userAgent = appName + "/" + version + " (+https://github.com/92mxs21/" + appName + ")"

// Downloader is a small parallel, resumable, checksum-verifying fetcher.
type Downloader struct {
	Jobs   int
	Client *http.Client
	Log    *Logger
}

func NewDownloader(jobs int, log *Logger) *Downloader {
	if jobs < 1 {
		jobs = 4
	}
	tr := &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Downloader{
		Jobs:   jobs,
		Client: &http.Client{Transport: tr},
		Log:    log,
	}
}

// DownloadTask is one file to fetch and verify.
type DownloadTask struct {
	URL  string
	Dest string
	SHA1 string
	Size int64
}

type dlResult struct {
	task   DownloadTask
	status string // "ok" | "skip" | "fail"
	bytes  int64
	err    error
}

// GetJSON performs a GET and decodes the body as JSON.
func (d *Downloader) GetJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d for %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (d *Downloader) verify(t DownloadTask) bool {
	fi, err := os.Stat(t.Dest)
	if err != nil || fi.IsDir() {
		return false
	}
	if t.Size > 0 && fi.Size() != t.Size {
		return false
	}
	if t.SHA1 != "" {
		sum, err := sha1File(t.Dest)
		if err != nil || !strings.EqualFold(sum, t.SHA1) {
			return false
		}
	}
	return true
}

func (d *Downloader) fetch(ctx context.Context, t DownloadTask) dlResult {
	if d.verify(t) {
		return dlResult{task: t, status: "skip"}
	}
	if err := ensureDir(filepath.Dir(t.Dest)); err != nil {
		return dlResult{task: t, status: "fail", err: err}
	}
	part := t.Dest + ".part"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return dlResult{task: t, status: "fail", err: ctx.Err()}
		}
		written, err := d.downloadOnce(ctx, t.URL, part)
		if err != nil {
			lastErr = err
			_ = os.Remove(part)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		if t.Size > 0 && written != t.Size {
			lastErr = fmt.Errorf("size %d != %d", written, t.Size)
			_ = os.Remove(part)
			continue
		}
		if t.SHA1 != "" {
			sum, err := sha1File(part)
			if err != nil || !strings.EqualFold(sum, t.SHA1) {
				lastErr = errors.New("sha1 mismatch")
				_ = os.Remove(part)
				continue
			}
		}
		if err := os.Rename(part, t.Dest); err != nil {
			lastErr = err
			continue
		}
		return dlResult{task: t, status: "ok", bytes: written}
	}
	_ = os.Remove(part)
	return dlResult{task: t, status: "fail", err: lastErr}
}

func (d *Downloader) downloadOnce(ctx context.Context, url, dest string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := d.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("http %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		return n, err
	}
	return n, f.Sync()
}

// Run downloads every task with a bounded worker pool and returns any failures.
func (d *Downloader) Run(ctx context.Context, label string, tasks []DownloadTask) []string {
	if len(tasks) == 0 {
		d.Log.Printf("    %-12s nichts zu tun", label)
		return nil
	}
	start := time.Now()
	var ok, skip, fail int64
	var bytes int64
	var failures []string

	jobCh := make(chan DownloadTask)
	resCh := make(chan dlResult)
	var wg sync.WaitGroup

	workers := d.Jobs
	if workers > len(tasks) {
		workers = len(tasks)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range jobCh {
				resCh <- d.fetch(ctx, t)
			}
		}()
	}
	go func() {
		for _, t := range tasks {
			jobCh <- t
		}
		close(jobCh)
	}()
	go func() {
		wg.Wait()
		close(resCh)
	}()

	total := int64(len(tasks))
	var done int64
	lastPrint := time.Now()
	for r := range resCh {
		done++
		switch r.status {
		case "ok":
			ok++
		case "skip":
			skip++
		default:
			fail++
			failures = append(failures, fmt.Sprintf("%s: %s (%v)", label, r.task.Dest, r.err))
		}
		bytes += r.bytes
		if time.Since(lastPrint) > 400*time.Millisecond || done == total {
			d.Log.Progress(fmt.Sprintf("%-12s %d/%d  (%d geladen, %d vorhanden, %d Fehler)",
				label, done, total, ok, skip, fail))
			lastPrint = time.Now()
		}
	}
	d.Log.ProgressEnd()
	d.Log.Printf("    %-12s %d Dateien -> %d geladen, %d vorhanden, %d Fehler  [%s, %s]",
		label, total, ok, skip, fail, time.Since(start).Round(time.Second), humanBytes(bytes))
	return failures
}
