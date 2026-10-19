package ui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProgressString(t *testing.T) {
	for _, tc := range []struct {
		p    Progress
		want string
	}{
		{Progress{Label: "Downloading x", Done: 3 << 20}, "Downloading x: 3 MiB"},
		{Progress{Label: "Downloading x", Done: 1 << 20, Total: 4 << 20}, "Downloading x: 1 MiB of 4 MiB (25%)"},
		{Progress{Label: "Downloading x", Done: 1 << 20, Total: 4 << 20, Rate: 1 << 20}, "Downloading x: 1 MiB of 4 MiB (25%), 1.0 MiB/s, about 3s left"},
		{Progress{Label: "Downloading x", Done: 1 << 20, Total: 601 << 20, Rate: 1 << 20}, "Downloading x: 1 MiB of 601 MiB (0%), 1.0 MiB/s, about 10m left"},
		{Progress{Label: "Downloading x", Done: 5 << 20, Rate: 2 << 20}, "Downloading x: 5 MiB, 2.0 MiB/s"},
	} {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
	if f := (Progress{Done: 9, Total: 4}).Fraction(); f != 1 {
		t.Errorf("Fraction past the end = %v", f)
	}
	if f := (Progress{Done: 9}).Fraction(); f != -1 {
		t.Errorf("Fraction with no total = %v", f)
	}
	if b := bar(0.5, 4); b != "██░░" {
		t.Errorf("bar = %q", b)
	}
}

func TestSpinProgressTerminal(t *testing.T) {
	var out bytes.Buffer
	u := NewTerminal(strings.NewReader(""), &out)
	shown := make(chan struct{})
	calls := 0
	progress := func() (Progress, bool) {
		calls++
		if calls == 2 {
			close(shown)
		}
		return Progress{Label: "Downloading ubuntu.img", Done: 1 << 20, Total: 2 << 20}, true
	}
	err := u.SpinProgress(context.Background(), "Creating dev", progress, func(context.Context) error {
		<-shown // a progress line has been read and drawn
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Downloading ubuntu.img: 1 MiB of 2 MiB (50%)") || !strings.Contains(out.String(), "█") {
		t.Errorf("no progress line in %q", out.String())
	}

	boom := errors.New("boom")
	if err := u.SpinProgress(context.Background(), "step", func() (Progress, bool) { return Progress{}, false },
		func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestSpinProgressPlain(t *testing.T) {
	was := PlainProgressEvery
	PlainProgressEvery = 10 * time.Millisecond
	t.Cleanup(func() { PlainProgressEvery = was })

	var out bytes.Buffer
	u := NewPlain(&out)
	progress := func() (Progress, bool) { return Progress{Label: "Downloading ubuntu.img", Done: 7 << 20}, true }
	if err := u.SpinProgress(context.Background(), "Creating dev", progress, func(context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "› Creating dev") || !strings.Contains(out.String(), "Downloading ubuntu.img: 7 MiB") {
		t.Errorf("plain output %q", out.String())
	}

	// Nothing to show, nothing printed but the title.
	out.Reset()
	boom := errors.New("boom")
	if err := u.SpinProgress(context.Background(), "step", func() (Progress, bool) { return Progress{}, false }, func(context.Context) error {
		time.Sleep(30 * time.Millisecond)
		return boom
	}); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Errorf("plain output with no progress %q", out.String())
	}
}
