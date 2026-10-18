package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
)

// claudePlan describes a project's Claude plugins and skills for up's plan.
func claudePlan(c project.Claude) []string {
	var plan []string
	if len(c.Marketplaces) > 0 {
		names := make([]string, 0, len(c.Marketplaces))
		for name := range c.Marketplaces {
			names = append(names, name+" ("+c.Marketplaces[name]+")")
		}
		sort.Strings(names)
		plan = append(plan, "claude marketplaces: "+strings.Join(names, ", "))
	}
	if len(c.Plugins) > 0 {
		plan = append(plan, "claude plugins: "+strings.Join(c.Plugins, ", "))
	}
	if len(c.Skills) > 0 {
		plan = append(plan, "claude skills: "+strings.Join(c.Skills, ", "))
	}
	return plan
}

// upClaude gives Claude Code in the VM the project's marketplaces, plugins
// and skills. The list goes over stdin as base64 JSON, so nothing from the
// file reaches a command line; the VM fetches everything itself.
func (a *App) upClaude(ctx context.Context, inst lima.Instance, c project.Claude) {
	if c.Empty() {
		return
	}
	data, _ := json.Marshal(map[string]any{
		"marketplaces": c.Marketplaces,
		"plugins":      c.Plugins,
		"skills":       c.Skills,
	})
	script := "DATA = \"" + base64.StdEncoding.EncodeToString(data) + "\"\n" + claudeScript
	a.UI.Info("Adding Claude plugins and skills in %s", a.UI.Code(inst.Name))
	if out, err := a.Lima.Run(ctx, inst, bytes.NewReader([]byte(script)), "python3 -"); err != nil {
		a.UI.Warn("Claude plugins and skills: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
		a.UI.Hint("A VM with a network allowlist needs github: limabean network allow "+inst.Name+" github",
			"Then: limabean rm "+inst.Name+" && limabean up")
		return
	}
	a.UI.OK("Claude Code in %s gets: %s", inst.Name, claudeSummary(c))
}

// claudeSummary lists what a project gives Claude, briefly.
func claudeSummary(c project.Claude) string {
	var parts []string
	if n := len(c.Plugins); n > 0 {
		parts = append(parts, fmt.Sprintf("%d plugin%s", n, plural(n)))
	}
	if n := len(c.Skills); n > 0 {
		parts = append(parts, fmt.Sprintf("%d skill%s", n, plural(n)))
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d marketplace%s", len(c.Marketplaces), plural(len(c.Marketplaces))))
	}
	return strings.Join(parts, " and ")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// claudeScript runs in the VM with python3, after a line setting DATA. It
// adds the marketplaces and plugins to ~/.claude/settings.json, where
// Claude Code (the claude pack's, or Claude Desktop's over SSH) fetches
// them when a session starts, and copies each skill's directory from its
// GitHub repository into ~/.claude/skills.
const claudeScript = `import base64, json, os, shutil, subprocess, sys, tempfile

data = json.loads(base64.b64decode(DATA))
claude = os.path.expanduser("~/.claude")
os.makedirs(claude, exist_ok=True)

path = os.path.join(claude, "settings.json")
try:
    with open(path) as f:
        settings = json.load(f)
except FileNotFoundError:
    settings = {}
except ValueError:
    sys.exit(path + " isn't valid JSON, so no plugins were added")
known = settings.setdefault("extraKnownMarketplaces", {})
for name, src in data["marketplaces"].items():
    if src.startswith("https://"):
        known[name] = {"source": {"source": "git", "url": src}}
    else:
        known[name] = {"source": {"source": "github", "repo": src}}
enabled = settings.setdefault("enabledPlugins", {})
for plugin in data["plugins"]:
    enabled[plugin] = True
tmp = path + ".tmp"
with open(tmp, "w") as f:
    json.dump(settings, f, indent=2)
    f.write("\n")
os.replace(tmp, path)

env = dict(os.environ, GIT_TERMINAL_PROMPT="0")
failed = []
for skill in data["skills"]:
    spec, _, ref = skill.partition("@")
    owner, repo, sub = spec.split("/", 2)
    with tempfile.TemporaryDirectory() as tmp:
        clone = os.path.join(tmp, "repo")
        args = ["git", "clone", "--quiet", "--depth=1", "--filter=blob:none", "--sparse"]
        if ref:
            args += ["--branch", ref]
        args += ["https://github.com/%s/%s.git" % (owner, repo), clone]
        src = os.path.join(clone, sub)
        if (subprocess.run(args, env=env).returncode != 0
                or subprocess.run(["git", "-C", clone, "sparse-checkout", "set", "--", sub], env=env).returncode != 0
                or not os.path.isfile(os.path.join(src, "SKILL.md"))):
            failed.append(skill)
            continue
        dst = os.path.join(claude, "skills", sub.rsplit("/", 1)[-1])
        shutil.rmtree(dst, ignore_errors=True)
        shutil.copytree(src, dst, ignore=shutil.ignore_patterns(".git"))
if failed:
    sys.exit("couldn't install skills (no SKILL.md, or the repository or ref wasn't found): " + ", ".join(failed))
`
