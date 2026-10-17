// Package lima drives Lima through the limactl command, its stable
// interface. limabean doesn't import Lima's Go packages: they make no
// promise of a stable API, and users run whatever limactl they installed.
package lima

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Runner runs external programs; tests replace it.
type Runner interface {
	// Output runs name with args, feeding stdin, and returns stdout and
	// stderr combined when combined is true, or stdout alone otherwise.
	Output(ctx context.Context, stdin io.Reader, combined bool, name string, args ...string) ([]byte, error)
}

// ExecRunner runs real programs.
type ExecRunner struct{}

// Output implements Runner.
func (ExecRunner) Output(ctx context.Context, stdin io.Reader, combined bool, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	if combined {
		return cmd.CombinedOutput()
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && stderr.Len() > 0 {
		err = &CommandError{Err: err, Output: stderr.String()}
	}
	return out, err
}

// CommandError carries a failed command's output, for showing to the user.
type CommandError struct {
	Err    error
	Output string
}

func (e *CommandError) Error() string { return e.Err.Error() }
func (e *CommandError) Unwrap() error { return e.Err }

// Instance is one entry of `limactl list --format json`.
type Instance struct {
	Name          string `json:"name"`
	Status        string `json:"status"`
	Dir           string `json:"dir"`
	CPUs          int    `json:"cpus"`
	Memory        int64  `json:"memory"`
	Disk          int64  `json:"disk"` // bytes
	SSHConfigFile string `json:"sshConfigFile"`
	// Limabean is true for VMs built from limabean's templates, whose
	// provisioning writes to /etc/limabean/mise/.
	Limabean bool `json:"-"`
	// NetworkPolicy is true for VMs that mount their network policy from
	// the Mac at /mnt/host/limabean (created by limabean with it).
	NetworkPolicy bool `json:"-"`
}

// Running reports whether the VM is up.
func (i Instance) Running() bool { return i.Status == "Running" }

// MemoryGiB is the VM's memory in GiB.
func (i Instance) MemoryGiB() int64 { return i.Memory / (1 << 30) }

// Client runs limactl and ssh.
type Client struct {
	Runner  Runner
	Limactl string // "limactl" by default
}

// New returns a Client that runs real programs.
func New() *Client { return &Client{Runner: ExecRunner{}, Limactl: "limactl"} }

func (c *Client) limactl() string {
	if c.Limactl != "" {
		return c.Limactl
	}
	return "limactl"
}

// Version returns limactl's version, e.g. "2.2.1".
func (c *Client) Version(ctx context.Context) (string, error) {
	out, err := c.Runner.Output(ctx, nil, false, c.limactl(), "--version")
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "", errors.New("unexpected limactl --version output")
	}
	return f[len(f)-1], nil
}

// List returns every Lima instance. limactl prints one JSON object per line.
func (c *Client) List(ctx context.Context) ([]Instance, error) {
	out, err := c.Runner.Output(ctx, nil, false, c.limactl(), "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("limactl list: %w", err)
	}
	return parseList(out)
}

func parseList(out []byte) ([]Instance, error) {
	var list []Instance
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var inst Instance
		if err := json.Unmarshal(line, &inst); err != nil {
			return nil, fmt.Errorf("parsing limactl list: %w", err)
		}
		inst.Limabean = bytes.Contains(line, []byte("/etc/limabean/mise/"))
		inst.NetworkPolicy = bytes.Contains(line, []byte(`"/mnt/host/limabean"`))
		list = append(list, inst)
	}
	return list, sc.Err()
}

// Limabean returns the VMs built from limabean's templates.
func (c *Client) Limabean(ctx context.Context) ([]Instance, error) {
	all, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []Instance
	for _, i := range all {
		if i.Limabean {
			out = append(out, i)
		}
	}
	return out, nil
}

// Get returns one instance, or ok == false if it doesn't exist.
func (c *Client) Get(ctx context.Context, name string) (inst Instance, ok bool, err error) {
	all, err := c.List(ctx)
	if err != nil {
		return Instance{}, false, err
	}
	for _, i := range all {
		if i.Name == name {
			return i, true, nil
		}
	}
	return Instance{}, false, nil
}

// StartOptions configure a new VM.
type StartOptions struct {
	Name     string
	Template string            // path of the template file
	Params   map[string]string // --param key=value
	CPUs     int               // 0: template default
	Memory   string            // GiB; "": template default
	Disk     int               // GiB of the system disk; 0: template default
	Sets     []string          // --set yq expressions
}

// Args returns the limactl arguments for creating the VM.
func (o StartOptions) Args() []string {
	args := []string{"start", "--name=" + o.Name, "--tty=false"}
	for _, k := range sortedKeys(o.Params) {
		args = append(args, "--param", k+"="+o.Params[k])
	}
	if o.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprint(o.CPUs))
	}
	if o.Memory != "" {
		args = append(args, "--memory", o.Memory)
	}
	if o.Disk > 0 {
		args = append(args, "--disk", fmt.Sprint(o.Disk))
	}
	for _, s := range o.Sets {
		args = append(args, "--set", s)
	}
	return append(args, o.Template)
}

// Create creates and boots a VM. Lima's output is returned for showing if
// something fails.
func (c *Client) Create(ctx context.Context, o StartOptions) ([]byte, error) {
	return c.Runner.Output(ctx, nil, true, c.limactl(), o.Args()...)
}

// Start boots an existing VM.
func (c *Client) Start(ctx context.Context, name string) ([]byte, error) {
	return c.Runner.Output(ctx, nil, true, c.limactl(), "start", "--tty=false", name)
}

// Stop shuts a VM down.
func (c *Client) Stop(ctx context.Context, name string) ([]byte, error) {
	return c.Runner.Output(ctx, nil, true, c.limactl(), "stop", name)
}

// Delete removes a VM, stopping it first if needed.
func (c *Client) Delete(ctx context.Context, name string) ([]byte, error) {
	return c.Runner.Output(ctx, nil, true, c.limactl(), "delete", "--force", name)
}

// Clone copies a stopped VM to a new one, with APFS clones of its disks,
// changing the copy's configuration with --set yq expressions, and with
// start boots the copy too (Lima 2.0's clone --start).
func (c *Client) Clone(ctx context.Context, start bool, src, dst string, sets ...string) ([]byte, error) {
	args := []string{"clone", "--tty=false"}
	if start {
		args = append(args, "--start")
	}
	for _, s := range sets {
		args = append(args, "--set", s)
	}
	return c.Runner.Output(ctx, nil, true, c.limactl(), append(args, src, dst)...)
}

// Copy copies files between the Mac and VMs with limactl copy: VM paths
// are written <vm>:<path>.
func (c *Client) Copy(ctx context.Context, recursive bool, paths ...string) ([]byte, error) {
	args := []string{"copy"}
	if recursive {
		args = append(args, "-r")
	}
	return c.Runner.Output(ctx, nil, true, c.limactl(), append(append(args, "--"), paths...)...)
}

// DiskExists reports whether a Lima disk (limactl disk create) exists.
func (c *Client) DiskExists(ctx context.Context, name string) (bool, error) {
	out, err := c.Runner.Output(ctx, nil, false, c.limactl(), "disk", "list", "--json")
	if err != nil {
		return false, fmt.Errorf("limactl disk list: %w", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		var d struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(sc.Bytes(), &d) == nil && d.Name == name {
			return true, nil
		}
	}
	return false, sc.Err()
}

// DiskCreate creates a raw disk of size (e.g. "50GiB"); vz converts other
// formats to raw anyway.
func (c *Client) DiskCreate(ctx context.Context, name, size string) ([]byte, error) {
	return c.Runner.Output(ctx, nil, true, c.limactl(), "disk", "create", name, "--size", size, "--format", "raw")
}

// DiskDelete deletes a disk that no VM is using.
func (c *Client) DiskDelete(ctx context.Context, name string) ([]byte, error) {
	return c.Runner.Output(ctx, nil, true, c.limactl(), "disk", "delete", name)
}

// SSHArgs returns the ssh arguments for a VM. They use the VM's own
// ssh.config, so they work whether or not ~/.ssh/config includes Lima's
// host entries, and whatever LIMA_HOME is.
func SSHArgs(inst Instance, batch bool, command ...string) []string {
	args := []string{"-F", inst.SSHConfigFile}
	if batch {
		args = append(args, "-o", "BatchMode=yes")
	}
	args = append(args, "lima-"+inst.Name)
	return append(args, command...)
}

// Run runs a shell command in a running VM non-interactively and returns its
// stdout. stdin, if not nil, is fed to the command.
func (c *Client) Run(ctx context.Context, inst Instance, stdin io.Reader, command string) ([]byte, error) {
	return c.Runner.Output(ctx, stdin, false, "ssh", SSHArgs(inst, true, command)...)
}

// Exec replaces limabean with an interactive ssh session in the VM.
func Exec(inst Instance, command ...string) error {
	path, err := exec.LookPath("ssh")
	if err != nil {
		return err
	}
	return execve(path, append([]string{"ssh"}, SSHArgs(inst, false, command...)...), os.Environ())
}

func sortedKeys(m map[string]string) []string {
	return slices.Sorted(maps.Keys(m))
}
