package cli

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rossedman/limabean/internal/ui"
)

// Lima downloads a VM's image the first time a template uses it, into its
// cache as download/by-url-sha256/<hash>/data.tmp.<pid>.*, with the URL
// beside it in "url". It prints no progress without a terminal, and starts
// over when interrupted, so a step that's downloading shows how far it's got
// from the size of that file.

// downloadActive is how recently a download's file must have grown for it
// to count as under way: older ones are left over from interrupted runs.
const downloadActive = 10 * time.Second

// limaCacheDir is Lima's cache directory: ~/Library/Caches/lima on macOS.
func (a *App) limaCacheDir() string {
	if a.limaCache != "" {
		return a.limaCache
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "lima")
}

// downloadWatch follows the download Lima has under way, if any.
type downloadWatch struct {
	a *App

	mu    sync.Mutex
	sizes map[string]int64 // URL: its length, 0 while asked for or unknown
	file  string           // the file last seen, and its size then
	size  int64
	at    time.Time
	rate  float64 // bytes a second, smoothed
}

// newDownloadWatch returns a watcher with no download seen yet.
func (a *App) newDownloadWatch() *downloadWatch {
	return &downloadWatch{a: a, sizes: map[string]int64{}}
}

// progress reports on the download Lima has under way; false when there's
// none.
func (w *downloadWatch) progress() (ui.Progress, bool) {
	cache := w.a.limaCacheDir()
	if cache == "" {
		return ui.Progress{}, false
	}
	files, _ := filepath.Glob(filepath.Join(cache, "download", "by-url-sha256", "*", "data.tmp.*"))
	var file string
	var info os.FileInfo
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil || time.Since(fi.ModTime()) > downloadActive {
			continue
		}
		if info == nil || fi.ModTime().After(info.ModTime()) {
			file, info = f, fi
		}
	}
	if info == nil {
		return ui.Progress{}, false
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(file), "url"))
	url := strings.TrimSpace(string(b))
	if err != nil || url == "" {
		return ui.Progress{}, false
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	now, size := time.Now(), info.Size()
	if file == w.file && size > w.size {
		if dt := now.Sub(w.at).Seconds(); dt > 0 {
			r := float64(size-w.size) / dt
			if w.rate == 0 {
				w.rate = r
			} else {
				w.rate = 0.8*w.rate + 0.2*r
			}
		}
	}
	if file != w.file {
		w.rate = 0
	}
	if file != w.file || size > w.size {
		w.file, w.size, w.at = file, size, now
	}
	total, asked := w.sizes[url]
	if !asked {
		w.sizes[url] = 0
		go w.askSize(url)
	}
	return ui.Progress{
		Label: "Downloading " + path.Base(url) + " (first time only)",
		Done:  size, Total: total, Rate: w.rate,
	}, true
}

// askSize looks up how big url is, for the progress bar.
func (w *downloadWatch) askSize(url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := w.a.contentLength(ctx, url)
	if err != nil || n <= 0 {
		return
	}
	w.mu.Lock()
	w.sizes[url] = n
	w.mu.Unlock()
}

// contentLength asks an image's server how big it is, with a HEAD request.
// Only https URLs are asked: the URL is one of the templates' own images.
func contentLength(ctx context.Context, url string) (int64, error) {
	if !strings.HasPrefix(url, "https://") {
		return 0, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}
	return resp.ContentLength, nil
}
