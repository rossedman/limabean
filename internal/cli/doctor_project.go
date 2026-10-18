package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/templates"
)

// doctorProject checks the project file of the checkout doctor runs in:
// what the schema can't know, because it depends on this Mac. It reports
// whether all passed; outside a project there's nothing to check.
func (a *App) doctorProject(ctx context.Context) bool {
	path, err := project.Find(".")
	if errors.Is(err, project.ErrNotFound) {
		return true
	}
	a.UI.Section("Project", firstNonEmpty(shortPath(path), "here"))
	if err != nil {
		a.UI.Bad("%v", err)
		return false
	}
	f, err := project.Load(path)
	if err != nil {
		a.UI.Bad("%v", err)
		return false
	}
	ok := true
	fail := func(msg string, hints ...string) { a.UI.Bad("%s", msg); a.UI.Hint(hints...); ok = false }

	if root, err := a.templatesRoot(); err == nil && f.Env != "" {
		if envs, _ := templates.Envs(root); !slices.Contains(envs, f.Env) {
			fail(fmt.Sprintf("env %q isn't an environment here (there are: %s)", f.Env, strings.Join(envs, ", ")))
		}
	}
	if _, err := a.resolvePacks(nil, f.Packs, nil); err != nil {
		fail("packs: "+err.Error(), "limabean packs lists them")
	}
	if _, err := expandAllow(f.Network.Allow); err != nil {
		fail("network: " + err.Error())
	}
	if _, missing := a.projectSecrets(f.Secrets); len(missing) > 0 {
		var hints []string
		for _, n := range missing {
			hints = append(hints, "limabean secrets set "+n+" --vm "+f.Name)
		}
		fail("secrets not set up on this Mac: "+strings.Join(missing, ", "), hints...)
	}
	if ok {
		a.UI.OK("%s loads, and its packs, network and secrets are all here", shortPath(path))
	}
	if b, err := os.ReadFile(path); err == nil && !strings.Contains(string(b), "yaml-language-server") {
		a.UI.Hint("Your editor can check it as you type: https://rossedman.github.io/limabean/projects/#editor-support")
	}

	switch inst, exists, err := a.Lima.Get(ctx, f.Name); {
	case err != nil:
		// Lima's own section already says what's wrong.
	case !exists:
		a.UI.Hint("No VM yet: limabean up")
	default:
		own, _ := a.projectSecrets(f.Secrets)
		if c, err := a.upDiff(inst, f, own); err == nil && !c.empty() {
			a.UI.Warn("%s (%s) differs from the file in %d way(s): limabean up --preview", f.Name, strings.ToLower(inst.Status), len(c.lines))
		} else if err == nil {
			a.UI.OK("%s (%s) matches the file", f.Name, strings.ToLower(inst.Status))
		}
	}
	return ok
}
