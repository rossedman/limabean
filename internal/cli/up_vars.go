package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
)

// varsFile is a project's vars as the export lines env.sh
// sources from ~/.config/limabean/project.env.
func varsFile(env map[string]string) string {
	var b strings.Builder
	b.WriteString("# From vars: in the project's .limabean.yaml, written by limabean up.\n")
	for _, name := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "export %s=%s\n", name, secrets.ShellQuote(env[name]))
	}
	return b.String()
}

// writeVars gives a running VM a project's vars, or removes
// it when there's none, and warns if the VM's env.sh is too old to read it.
func (a *App) writeVars(ctx context.Context, inst lima.Instance, env map[string]string) error {
	cmd := `rm -f "$HOME/.config/limabean/project.env"`
	var stdin *strings.Reader
	if len(env) > 0 {
		cmd = `mkdir -p "$HOME/.config/limabean" && umask 077 && cat > "$HOME/.config/limabean/project.env.new" && mv "$HOME/.config/limabean/project.env.new" "$HOME/.config/limabean/project.env"`
		stdin = strings.NewReader(varsFile(env))
	}
	var err error
	if stdin != nil {
		_, err = a.Lima.Run(ctx, inst, stdin, cmd)
	} else {
		_, err = a.Lima.Run(ctx, inst, nil, cmd)
	}
	if err != nil {
		return fmt.Errorf("writing %s's vars: %w", inst.Name, err)
	}
	if len(env) > 0 {
		if _, err := a.Lima.Run(ctx, inst, nil, `[ -n "$(sed -n /project.env/p "$HOME/.config/limabean/env.sh")" ]`); err != nil {
			a.UI.Warn("%s was made before vars: worked, so its shells won't see it until it's rebuilt", inst.Name)
			a.UI.Hint("limabean rebuild " + inst.Name + "   (keeps ~/code)")
		}
	}
	return nil
}

// varsState holds, on the Mac, the vars last applied to a VM.
func (a *App) varsState(vm string) string {
	return filepath.Join(a.stateDir, vm+".vars")
}

func (a *App) appliedVars(vm string) map[string]string {
	env := map[string]string{}
	if b, err := os.ReadFile(a.varsState(vm)); err == nil {
		_ = json.Unmarshal(b, &env)
	}
	return env
}

func (a *App) saveVars(vm string, env map[string]string) error {
	if len(env) == 0 {
		if err := os.Remove(a.varsState(vm)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(env)
	return os.WriteFile(a.varsState(vm), b, 0o644)
}

// varsPlan shows a project's vars for up's plan.
func varsPlan(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	plan := []string{"vars:"}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		plan = append(plan, "  "+name+"="+env[name])
	}
	return plan
}

// varsChanges lists what differs between the vars a VM has
// and the file's.
func varsChanges(have, want map[string]string) []string {
	var lines []string
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if v, ok := have[name]; !ok || v != want[name] {
			lines = append(lines, "vars: set "+name+"="+want[name])
		}
	}
	for _, name := range slices.Sorted(maps.Keys(have)) {
		if _, ok := want[name]; !ok {
			lines = append(lines, "vars: unset "+name)
		}
	}
	return lines
}
