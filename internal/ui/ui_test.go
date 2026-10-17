package ui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPlainOutput(t *testing.T) {
	var out bytes.Buffer
	u := NewPlain(&out)
	u.OK("done %d", 1)
	u.Bad("broke")
	u.Warn("careful")
	u.Hint("one", "two")
	u.Info("step")
	u.Section("Title", "")
	u.Section("Title", "sub")
	u.Box("a", "b")
	u.Table([]string{"NAME", "STATUS"}, [][]string{{"dev", "Running"}})
	got := out.String()
	for _, want := range []string{"  ✓ done 1\n", "  ✗ broke\n", "  ! careful\n", "    one\n    two\n", "› step\n",
		"\nTitle\n", "\nTitle (sub)\n", "\na\nb\n", "NAME  STATUS\ndev   Running\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("plain output has escape codes:\n%q", got)
	}
	if u.Code("x") != "x" || u.Dimmed("y") != "y" {
		t.Error("Code/Dimmed should be plain")
	}
}

func TestTerminalOutput(t *testing.T) {
	var out bytes.Buffer
	u := NewTerminal(strings.NewReader(""), &out)
	u.Section("Title", "sub")
	u.Section("Only", "")
	u.Box("ready")
	u.Table([]string{"NAME"}, [][]string{{"dev"}})
	got := out.String()
	for _, want := range []string{"Title", "sub", "Only", "╭", "ready", "NAME", "dev"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestPromptsWithoutATerminal(t *testing.T) {
	u := NewPlain(&bytes.Buffer{})
	if v, err := u.Confirm("ok?", true); err != nil || !v {
		t.Errorf("Confirm default = %v, %v", v, err)
	}
	if v, err := u.Input("name", "dev", "", nil); err != nil || v != "dev" {
		t.Errorf("Input default = %q, %v", v, err)
	}
	if _, err := u.Password("token", ""); err == nil {
		t.Error("Password needs a terminal")
	}
	if _, err := u.Select("pick", []string{"a"}); err == nil {
		t.Error("Select needs a terminal")
	}
}

func TestFormsInATerminal(t *testing.T) {
	u := NewTerminal(Keys("y"), &bytes.Buffer{})
	if v, err := u.Confirm("ok?", false); err != nil || !v {
		t.Errorf("Confirm = %v, %v", v, err)
	}
	u = NewTerminal(Keys(" box ", Enter), &bytes.Buffer{})
	if v, err := u.Input("name", "", "", func(s string) error { return nil }); err != nil || v != "box" {
		t.Errorf("Input = %q, %v", v, err)
	}
	u = NewTerminal(Keys("s3cret", Enter), &bytes.Buffer{})
	if v, err := u.Password("token", ""); err != nil || v != "s3cret" {
		t.Errorf("Password = %q, %v", v, err)
	}
	u = NewTerminal(Keys(Down, Enter), &bytes.Buffer{})
	if v, err := u.Select("pick", []string{"a", "b"}); err != nil || v != "b" {
		t.Errorf("Select = %q, %v", v, err)
	}
	u = NewTerminal(Keys("\x03"), &bytes.Buffer{}) // ctrl-c
	if _, err := u.Confirm("ok?", false); !errors.Is(err, ErrAborted) {
		t.Errorf("ctrl-c: err = %v, want ErrAborted", err)
	}
}

func TestSpin(t *testing.T) {
	var out bytes.Buffer
	ran := false
	if err := NewPlain(&out).Spin(context.Background(), "plain step", func(context.Context) error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("plain spin: %v %v", err, ran)
	}
	if !strings.Contains(out.String(), "› plain step") {
		t.Errorf("plain spin output: %q", out.String())
	}
	u := NewTerminal(strings.NewReader(""), &bytes.Buffer{})
	boom := errors.New("boom")
	if err := u.Spin(context.Background(), "step", func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("terminal spin err = %v, want boom", err)
	}
	if err := u.Spin(context.Background(), "step", func(context.Context) error { return nil }); err != nil {
		t.Errorf("terminal spin: %v", err)
	}
}

func TestNewAndTheme(t *testing.T) {
	if u := New(true); u.Interactive || u.Prompting {
		t.Error("New(plain) should not be interactive")
	}
	if Theme() == nil {
		t.Error("Theme")
	}
}

func TestKeysSplitsTextButNotEscapes(t *testing.T) {
	r := Keys("ab", Down, Enter).(*keyReader)
	if len(r.keys) != 4 || r.keys[2] != Down || r.keys[3] != Enter {
		t.Fatalf("keys = %q", r.keys)
	}
}
