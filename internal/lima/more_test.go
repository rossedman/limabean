package lima

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type ioReader = io.Reader

func TestExecRunner(t *testing.T) {
	r := ExecRunner{}
	ctx := context.Background()
	out, err := r.Output(ctx, strings.NewReader("in"), false, "sh", "-c", "cat; echo out")
	if err != nil || string(out) != "inout\n" {
		t.Fatalf("stdout = %q, %v", out, err)
	}
	out, err = r.Output(ctx, nil, true, "sh", "-c", "echo a; echo b >&2")
	if err != nil || !strings.Contains(string(out), "a") || !strings.Contains(string(out), "b") {
		t.Fatalf("combined = %q, %v", out, err)
	}
	_, err = r.Output(ctx, nil, false, "sh", "-c", "echo boom >&2; exit 3")
	var ce *CommandError
	if !errors.As(err, &ce) || !strings.Contains(ce.Output, "boom") || ce.Error() == "" || ce.Unwrap() == nil {
		t.Fatalf("err = %#v", err)
	}
	if _, err := r.Output(ctx, nil, false, "sh", "-c", "exit 4"); err == nil || errors.As(err, &ce) {
		t.Fatalf("silent failure should be a plain error, got %#v", err)
	}
}

func TestLifecycleCommands(t *testing.T) {
	f := &fakeRunner{out: map[string]string{"--version": "limactl version 2.2.1\n"}}
	c := &Client{Runner: f, Limactl: "/opt/limactl"}
	ctx := context.Background()
	if v, err := c.Version(ctx); err != nil || v != "2.2.1" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	_, _ = c.Create(ctx, StartOptions{Name: "dev", Template: "t.yaml"})
	_, _ = c.Start(ctx, "dev")
	_, _ = c.Stop(ctx, "dev")
	_, _ = c.Delete(ctx, "dev")
	got := make([]string, len(f.calls))
	for i, call := range f.calls {
		got[i] = strings.Join(call, " ")
	}
	want := []string{
		"/opt/limactl --version",
		"/opt/limactl start --name=dev --tty=false t.yaml",
		"/opt/limactl start --tty=false dev",
		"/opt/limactl stop dev",
		"/opt/limactl delete --force dev",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if New().limactl() != "limactl" || (&Client{}).limactl() != "limactl" {
		t.Fatal("default limactl")
	}
}

type errRunner struct{ out string }

func (e errRunner) Output(context.Context, ioReader, bool, string, ...string) ([]byte, error) {
	return []byte(e.out), errors.New("failed")
}

func TestErrors(t *testing.T) {
	c := &Client{Runner: errRunner{}}
	ctx := context.Background()
	if _, err := c.Version(ctx); err == nil {
		t.Error("Version should fail")
	}
	if _, err := c.List(ctx); err == nil {
		t.Error("List should fail")
	}
	if _, err := c.Limabean(ctx); err == nil {
		t.Error("Limabean should fail")
	}
	if _, _, err := c.Get(ctx, "x"); err == nil {
		t.Error("Get should fail")
	}
	if _, err := (&Client{Runner: &fakeRunner{out: map[string]string{"--version": "  "}}}).Version(ctx); err == nil {
		t.Error("empty version output should fail")
	}
	if _, err := parseList([]byte("{not json\n")); err == nil {
		t.Error("bad JSON should fail")
	}
}

func TestSSHArgsInteractive(t *testing.T) {
	got := strings.Join(SSHArgs(Instance{Name: "dev", SSHConfigFile: "/c"}, false, "ls", "-la"), " ")
	if got != "-F /c lima-dev ls -la" {
		t.Fatalf("got %q", got)
	}
}
