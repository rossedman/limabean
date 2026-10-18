package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	for err, want := range map[error]int{
		usageErr("x"):    exitUsage,
		notFoundErr("x"): exitNotFound,
		conflictErr("x"): exitConflict,
		errUnsaved:       exitConflict,
		errors.New(`unknown command "x" for "limabean"`):   exitUsage,
		errors.New(`accepts at most 1 arg(s), received 2`): exitUsage,
		errors.New("limactl failed"):                       exitError,
	} {
		if code, _ := classify(err); code != want {
			t.Errorf("%v: %d, want %d", err, code, want)
		}
	}
	if !wantsJSON([]string{"ls", "--json"}) || wantsJSON([]string{"run", "--", "x", "--json"}) || wantsJSON([]string{"ls"}) {
		t.Error("wantsJSON")
	}
}

func TestJSONErrorAndBugReport(t *testing.T) {
	var b bytes.Buffer
	writeJSONError(&b, notFoundErr("no VM named %q", "x"))
	var got struct {
		Error struct {
			Code    string `json:"code"`
			Exit    int    `json:"exit"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b.Bytes(), &got); err != nil || got.Error.Code != "not_found" || got.Error.Exit != 4 || got.Error.Message != `no VM named "x"` {
		t.Fatalf("json error: %v %s", err, b.String())
	}
	b.Reset()
	bugReport(&b, "1.2.3", []string{"up", "dev"}, "boom")
	for _, want := range []string{"bug in limabean", "issues/new", "version: 1.2.3", "command: limabean up dev", "error:   boom"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("bug report missing %q", want)
		}
	}
}

// Commands report the documented codes.
func TestCommandExitKinds(t *testing.T) {
	h := newHarness(t)
	t.Setenv("LIMABEAN_CODE_DISK", "")
	for args, want := range map[string]int{
		"ssh nope":      exitNotFound,
		"down nope":     exitNotFound,
		"up bad_name":   exitUsage,
		"ls a b":        exitUsage,
		"down --idle 1": exitUsage,
	} {
		_, err := h.run("", strings.Fields(args)...)
		if err == nil {
			t.Errorf("%s: no error", args)
			continue
		}
		if code, _ := classify(err); code != want {
			t.Errorf("%s: exit %d (%v), want %d", args, code, err, want)
		}
	}
	h.must("", "up", "dev")
	for args, want := range map[string]int{
		"clone dev dev": exitConflict,
		"new dev":       exitConflict,
	} {
		if _, err := h.run("", strings.Fields(args)...); err == nil {
			t.Errorf("%s: no error", args)
		} else if code, _ := classify(err); code != want {
			t.Errorf("%s: exit %d (%v), want %d", args, code, err, want)
		}
	}
	h.world.unsaved = "~/code/x: 1 unpushed commit\n"
	if _, err := h.run("", "rm", "dev"); !errors.Is(err, errUnsaved) {
		t.Errorf("rm with unsaved work: %v", err)
	}
}
