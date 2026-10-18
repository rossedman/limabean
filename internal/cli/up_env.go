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

// environmentFile is a project's environment as the export lines env.sh
// sources from ~/.config/limabean/project.env.
func environmentFile(env map[string]string) string {
	var b strings.Builder
	b.WriteString("# From environment: in the project's .limabean.yaml, written by limabean up.\n")
	for _, name := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "export %s=%s\n", name, secrets.ShellQuote(env[name]))
	}
	return b.String()
}

// writeEnvironment gives a running VM a project's environment, or removes
// it when there's none, and warns if the VM's env.sh is too old to read it.
func (a *App) writeEnvironment(ctx context.Context, inst lima.Instance, env map[string]string) error {
	cmd := `rm -f "$HOME/.config/limabean/project.env"`
	var stdin *strings.Reader
	if len(env) > 0 {
		cmd = `mkdir -p "$HOME/.config/limabean" && umask 077 && cat > "$HOME/.config/limabean/project.env.new" && mv "$HOME/.config/limabean/project.env.new" "$HOME/.config/limabean/project.env"`
		stdin = strings.NewReader(environmentFile(env))
	}
	var err error
	if stdin != nil {
		_, err = a.Lima.Run(ctx, inst, stdin, cmd)
	} else {
		_, err = a.Lima.Run(ctx, inst, nil, cmd)
	}
	if err != nil {
		return fmt.Errorf("writing %s's environment: %w", inst.Name, err)
	}
	if len(env) > 0 {
		if _, err := a.Lima.Run(ctx, inst, nil, `[ -n "$(sed -n /project.env/p "$HOME/.config/limabean/env.sh")" ]`); err != nil {
			a.UI.Warn("%s was made before environment: worked, so its shells won't see it until it's rebuilt", inst.Name)
			a.UI.Hint("limabean rebuild " + inst.Name + "   (keeps ~/code)")
		}
	}
	return nil
}

// environmentState holds, on the Mac, the environment last applied to a VM.
func (a *App) environmentState(vm string) string {
	return filepath.Join(a.stateDir, vm+".environment")
}

func (a *App) appliedEnvironment(vm string) map[string]string {
	env := map[string]string{}
	if b, err := os.ReadFile(a.environmentState(vm)); err == nil {
		_ = json.Unmarshal(b, &env)
	}
	return env
}

func (a *App) saveEnvironment(vm string, env map[string]string) error {
	if len(env) == 0 {
		if err := os.Remove(a.environmentState(vm)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(a.stateDir, 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(env)
	return os.WriteFile(a.environmentState(vm), b, 0o644)
}

// environmentPlan shows a project's environment for up's plan.
func environmentPlan(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	plan := []string{"environment:"}
	for _, name := range slices.Sorted(maps.Keys(env)) {
		plan = append(plan, "  "+name+"="+env[name])
	}
	return plan
}

// environmentChanges lists what differs between the environment a VM has
// and the file's.
func environmentChanges(have, want map[string]string) []string {
	var lines []string
	for _, name := range slices.Sorted(maps.Keys(want)) {
		if v, ok := have[name]; !ok || v != want[name] {
			lines = append(lines, "environment: set "+name+"="+want[name])
		}
	}
	for _, name := range slices.Sorted(maps.Keys(have)) {
		if _, ok := want[name]; !ok {
			lines = append(lines, "environment: unset "+name)
		}
	}
	return lines
}
