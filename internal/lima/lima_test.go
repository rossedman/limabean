package lima

import (
	"context"
	"io"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls  [][]string
	stdins []string
	out    map[string]string // first arg after the program → output
}

func (f *fakeRunner) Output(_ context.Context, stdin io.Reader, _ bool, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	in := ""
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		in = string(b)
	}
	f.stdins = append(f.stdins, in)
	if len(args) > 0 {
		return []byte(f.out[args[0]]), nil
	}
	return nil, nil
}

const listJSON = `{"name":"dev","status":"Running","dir":"/h/.lima/dev","cpus":4,"memory":8589934592,"sshConfigFile":"/h/.lima/dev/ssh.config","config":{"mounts":[{"mountPoint":"/mnt/host/limabean"}],"provision":[{"mode":"data","path":"/etc/limabean/mise/00-base.toml"}]}}
{"name":"other","status":"Stopped","dir":"/h/.lima/other","cpus":2,"memory":4294967296,"sshConfigFile":"/h/.lima/other/ssh.config","config":{}}
time="2026-10-16T19:00:00-05:00" level=warning msg="noise that isn't JSON"
`

func TestListAndLimabean(t *testing.T) {
	c := &Client{Runner: &fakeRunner{out: map[string]string{"list": listJSON}}}
	all, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Name != "dev" || !all[0].Running() || all[0].MemoryGiB() != 8 || !all[0].Limabean || all[1].Limabean || !all[0].NetworkPolicy || all[1].NetworkPolicy {
		t.Fatalf("got %+v", all)
	}
	lb, _ := c.Limabean(context.Background())
	if len(lb) != 1 || lb[0].Name != "dev" {
		t.Fatalf("Limabean = %+v", lb)
	}
	inst, ok, _ := c.Get(context.Background(), "other")
	if !ok || inst.CPUs != 2 {
		t.Fatalf("Get other = %+v %v", inst, ok)
	}
	if _, ok, _ := c.Get(context.Background(), "nope"); ok {
		t.Fatal("Get nope found something")
	}
}

func TestStartArgs(t *testing.T) {
	o := StartOptions{
		Name: "dev", Template: "/t/scratch.yaml",
		Params: map[string]string{"gitConfigDir": "~/.config/git", "codeDir": "~/code"},
		CPUs:   6, Memory: "12",
		Sets: []string{`.mounts += [{"location":"~/.aws"}]`},
	}
	got := strings.Join(o.Args(), " ")
	want := `start --name=dev --tty=false --param codeDir=~/code --param gitConfigDir=~/.config/git --cpus 6 --memory 12 --set .mounts += [{"location":"~/.aws"}] /t/scratch.yaml`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if args := (StartOptions{Name: "x", Template: "t.yaml"}).Args(); strings.Join(args, " ") != "start --name=x --tty=false t.yaml" {
		t.Fatalf("minimal args = %v", args)
	}
}

func TestRunUsesVMsSSHConfigAndStdin(t *testing.T) {
	f := &fakeRunner{}
	c := &Client{Runner: f}
	inst := Instance{Name: "dev", SSHConfigFile: "/h/.lima/dev/ssh.config"}
	if _, err := c.Run(context.Background(), inst, strings.NewReader("secret"), "cat > f"); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(f.calls[0], " ")
	if got != "ssh -F /h/.lima/dev/ssh.config -o BatchMode=yes lima-dev cat > f" {
		t.Fatalf("ssh call = %s", got)
	}
	if f.stdins[0] != "secret" || strings.Contains(got, "secret") {
		t.Fatalf("secret must go on stdin, not argv: %q", got)
	}
}
