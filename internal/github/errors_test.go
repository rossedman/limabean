package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer forbidden":
			w.WriteHeader(http.StatusForbidden)
		case "Bearer down":
			w.WriteHeader(http.StatusBadGateway)
		default:
			_, _ = w.Write([]byte(`not json`))
		}
	}))
	defer srv.Close()
	c := Client{BaseURL: srv.URL}
	ctx := context.Background()
	if _, err := c.Check(ctx, "forbidden"); err != ErrRejected {
		t.Errorf("403: %v", err)
	}
	if _, err := c.Check(ctx, "down"); err == nil || !strings.Contains(err.Error(), "502") {
		t.Errorf("502: %v", err)
	}
	if _, err := c.Check(ctx, "garbage"); err == nil || !strings.Contains(err.Error(), "unexpected reply") {
		t.Errorf("bad JSON: %v", err)
	}
	if _, err := (Client{BaseURL: "http://127.0.0.1:1"}).Check(ctx, "x"); err == nil || !strings.Contains(err.Error(), "couldn't reach") {
		t.Errorf("unreachable: %v", err)
	}
	if _, err := (Client{BaseURL: "://bad"}).Check(ctx, "x"); err == nil {
		t.Error("bad URL should fail")
	}
}

func TestDefaultBaseURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // never actually reach api.github.com
	if _, err := (Client{}).Check(ctx, "x"); err == nil {
		t.Error("cancelled request should fail")
	}
}
