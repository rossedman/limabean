package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeDownload puts a download of url in Lima's cache under cache, size
// bytes so far, last written at mtime.
func fakeDownload(t *testing.T, cache, hash, url string, size int64, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(cache, "download", "by-url-sha256", hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "url"), []byte(url+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "data.tmp.123.1")
	if err := os.WriteFile(f, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(f, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDownloadWatch(t *testing.T) {
	cache := t.TempDir()
	asked := make(chan string, 1)
	a := &App{limaCache: cache, contentLength: func(_ context.Context, url string) (int64, error) {
		asked <- url
		return 4 << 20, nil
	}}
	w := a.newDownloadWatch()
	if _, ok := w.progress(); ok {
		t.Fatal("progress with nothing downloading")
	}

	// A download left over from an interrupted run isn't shown.
	fakeDownload(t, cache, "old", "https://example.com/old.img", 1<<20, time.Now().Add(-time.Hour))
	if _, ok := w.progress(); ok {
		t.Fatal("progress for a stale download")
	}

	url := "https://example.com/images/ubuntu.img"
	f := fakeDownload(t, cache, "new", url, 1<<20, time.Now())
	p, ok := w.progress()
	if !ok || p.Done != 1<<20 || p.Rate != 0 || !strings.Contains(p.Label, "ubuntu.img") {
		t.Fatalf("progress = %+v, %v", p, ok)
	}
	if got := <-asked; got != url {
		t.Errorf("asked the size of %q", got)
	}
	// The size arrives in the background; then the file grows.
	deadline := time.Now().Add(5 * time.Second)
	for {
		w.mu.Lock()
		n := w.sizes[url]
		w.mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	w.mu.Lock()
	w.at = w.at.Add(-time.Second)
	w.mu.Unlock()
	if err := os.WriteFile(f, make([]byte, 2<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	p, ok = w.progress()
	if !ok || p.Total != 4<<20 || p.Rate <= 0 || p.Fraction() != 0.5 {
		t.Fatalf("progress after growing = %+v, %v", p, ok)
	}
	if len(asked) != 0 {
		t.Error("asked for the size twice")
	}
}

func TestDownloadWatchNeedsURL(t *testing.T) {
	cache := t.TempDir()
	f := fakeDownload(t, cache, "x", "https://example.com/x.img", 10, time.Now())
	if err := os.Remove(filepath.Join(filepath.Dir(f), "url")); err != nil {
		t.Fatal(err)
	}
	a := &App{limaCache: cache, contentLength: func(context.Context, string) (int64, error) { return 0, errors.New("unused") }}
	if _, ok := a.newDownloadWatch().progress(); ok {
		t.Error("progress for a download with no URL")
	}
}

func TestLimaCacheDir(t *testing.T) {
	if got := (&App{limaCache: "/x"}).limaCacheDir(); got != "/x" {
		t.Errorf("limaCacheDir = %q", got)
	}
	if got := (&App{}).limaCacheDir(); filepath.Base(got) != "lima" {
		t.Errorf("default limaCacheDir = %q", got)
	}
}

func TestContentLength(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", "945530880")
	}))
	defer srv.Close()
	was := http.DefaultClient
	http.DefaultClient = srv.Client()
	t.Cleanup(func() { http.DefaultClient = was })

	ctx := t.Context()
	if n, err := contentLength(ctx, srv.URL+"/ubuntu.img"); err != nil || n != 945530880 {
		t.Errorf("contentLength = %d, %v", n, err)
	}
	if n, err := contentLength(ctx, srv.URL+"/missing"); err != nil || n != 0 {
		t.Errorf("missing: %d, %v", n, err)
	}
	if n, err := contentLength(ctx, "http://example.com/plain.img"); err != nil || n != 0 {
		t.Errorf("http URL asked: %d, %v", n, err)
	}
	if _, err := contentLength(ctx, "https://\x00"); err == nil {
		t.Error("bad URL: no error")
	}
	srv.Close()
	if _, err := contentLength(ctx, srv.URL+"/gone"); err == nil {
		t.Error("closed server: no error")
	}
}
