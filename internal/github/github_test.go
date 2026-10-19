package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer good":
			w.Header().Set("github-authentication-token-expiration", "2026-12-01 00:00:00 -0500")
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
		case "Bearer forever":
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	c := Client{BaseURL: srv.URL}
	ctx := context.Background()

	info, err := c.Check(ctx, "good")
	if err != nil || info.Login != "octocat" || info.Expires != "2026-12-01" {
		t.Fatalf("good: %+v, %v", info, err)
	}
	info, err = c.Check(ctx, "forever")
	if err != nil || info.Expires != "never" {
		t.Fatalf("forever: %+v, %v", info, err)
	}
	if _, err := c.Check(ctx, "bad"); !errors.Is(err, ErrRejected) {
		t.Fatalf("bad: err = %v, want ErrRejected", err)
	}
}

func TestFineGrained(t *testing.T) {
	if !FineGrained("github_pat_abc") || FineGrained("gho_abc") || FineGrained("ghp_abc") {
		t.Fatal("FineGrained misclassified a token")
	}
}

func TestLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/me/good/releases/latest":
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
		case "/repos/me/garbled/releases/latest":
			_, _ = w.Write([]byte(`not json`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := Client{BaseURL: srv.URL}
	ctx := context.Background()
	if tag, err := c.LatestRelease(ctx, "me/good"); err != nil || tag != "v1.2.3" {
		t.Errorf("good: %q, %v", tag, err)
	}
	for _, repo := range []string{"me/garbled", "me/none"} {
		if _, err := c.LatestRelease(ctx, repo); err == nil {
			t.Errorf("%s: no error", repo)
		}
	}
	if _, err := (Client{BaseURL: "http://127.0.0.1:1"}).LatestRelease(ctx, "me/good"); err == nil {
		t.Error("unreachable: no error")
	}
}
