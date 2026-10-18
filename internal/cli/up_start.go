package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/secrets"
)

// vmLogDir is where a VM keeps the output of a project's setup and start,
// for limabean logs.
const vmLogDir = "$HOME/.local/state/limabean"

// logged makes a remote command also write its output to <name>.log in
// vmLogDir, keeping its exit status.
func logged(command, name string) string {
	return `mkdir -p "` + vmLogDir + `" && set -o pipefail && { ` + command + `; } 2>&1 | tee "` + vmLogDir + `/` + name + `.log"`
}

// startScript runs a project's start commands in dir, detached, so they
// outlive the ssh session; a start still running from before is left
// alone. It prints "running" in that case. A finished one that nothing
// reaped yet (a zombie) doesn't count.
func startScript(dir, start string) string {
	cd := "cd"
	if dir != "" {
		cd += " " + secrets.ShellQuote(dir)
	}
	return `d="` + vmLogDir + `" && mkdir -p "$d" && ` +
		`if s=$(ps -o stat= -p "$(cat "$d/start.pid" 2>/dev/null)" 2>/dev/null) && [ -n "$s" ] && [ "${s#Z}" = "$s" ]; then echo running; exit 0; fi && ` +
		cd + ` && { setsid nohup bash -ec ` + secrets.ShellQuote(start) + ` >"$d/start.log" 2>&1 </dev/null & echo $! >"$d/start.pid"; }`
}

// runStart starts a project's start commands in the VM, unless they're
// already running.
func (a *App) runStart(ctx context.Context, inst lima.Instance, f *project.File, dir string) error {
	if f.Start == "" {
		return nil
	}
	out, err := a.Lima.Run(ctx, inst, nil, startScript(dir, f.Start))
	if err != nil {
		return fmt.Errorf("start: %w %s", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) == "running" {
		a.UI.Info("start is still running from before")
	} else {
		a.UI.OK("Started in the background: %s", firstOf(f.Start))
	}
	a.UI.Hint("Its output: limabean logs " + inst.Name + " --start -f")
	return nil
}

// projectDir is where a project's commands run in an existing VM: its
// clone, or the home directory when it has none.
func (a *App) projectDir(ctx context.Context, inst lima.Instance, f *project.File, dir string) string {
	repo := a.projectRepo(ctx, f, dir)
	if repo == "" {
		return ""
	}
	clone, err := a.codeDir(ctx, inst, repo)
	if err != nil {
		a.UI.Warn("%v; start runs in the home directory", err)
		return ""
	}
	return clone
}

// startFile holds the start commands last approved for a VM, on the Mac:
// a project file can change with a git pull, and commands that run on
// every up shouldn't change without the user seeing them.
func (a *App) startFile(vm string) string { return filepath.Join(a.stateDir, vm+".start") }

// approvedStart returns the start commands last approved for a VM.
func (a *App) approvedStart(vm string) string {
	b, err := os.ReadFile(a.startFile(vm))
	if err != nil {
		return ""
	}
	return string(b)
}

// approveStart remembers a VM's start commands as approved.
func (a *App) approveStart(vm, start string) error {
	if start == "" {
		if err := os.Remove(a.startFile(vm)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(a.startFile(vm), []byte(start), 0o644)
}

// startPlan shows start commands for a plan.
func startPlan(start string) []string {
	plan := []string{"start, run in the background after every up:"}
	for line := range strings.Lines(strings.TrimSpace(start)) {
		plan = append(plan, "  "+strings.TrimRight(line, "\n"))
	}
	return plan
}

// firstOf is a script's first line.
func firstOf(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
