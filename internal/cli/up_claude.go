package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/project"
)

// claudePlan describes a project's Claude plugins for up's plan, one line
// per marketplace.
func claudePlan(c project.Claude) []string {
	var plan []string
	for _, m := range c.Marketplaces {
		plan = append(plan, "claude plugins from "+m.Source+": "+strings.Join(m.Plugins, ", "))
	}
	return plan
}

// upClaude gives Claude Code in the VM the project's plugins. The list goes
// over stdin as base64 JSON, so nothing from the file reaches a command
// line on the Mac. In the VM, the script turns the plugins on in Claude
// Code's settings and, where the claude CLI is installed, installs them:
// a Claude Code session adds a settings file's marketplaces, but doesn't
// install its plugins.
func (a *App) upClaude(ctx context.Context, inst lima.Instance, c project.Claude) {
	if c.Empty() {
		return
	}
	marketplaces := map[string]string{}
	for _, m := range c.Marketplaces {
		marketplaces[m.Name()] = m.Source
	}
	data, _ := json.Marshal(map[string]any{"marketplaces": marketplaces, "plugins": c.Plugins()})
	script := "DATA = \"" + base64.StdEncoding.EncodeToString(data) + "\"\n" + claudeScript
	out, err := a.Lima.Run(ctx, inst, bytes.NewReader([]byte(script)), "python3 -")
	if err != nil {
		a.UI.Warn("Claude plugins not added: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
		return
	}
	var res struct {
		NoCLI  bool     `json:"no_cli"`
		Failed []string `json:"failed"`
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	_ = json.Unmarshal([]byte(lines[len(lines)-1]), &res)
	for _, f := range res.Failed {
		a.UI.Warn("Claude plugin not installed: %s", f)
	}
	switch {
	case res.NoCLI:
		a.UI.OK("Claude plugins for %s: %s (turned on; /plugin in a Claude session installs them)", inst.Name, strings.Join(c.Plugins(), ", "))
	case len(res.Failed) < len(c.Plugins()):
		a.UI.OK("Claude plugins for %s: %s", inst.Name, strings.Join(c.Plugins(), ", "))
	}
}

// claudeScript runs in the VM with python3, after a line setting DATA. It
// adds the marketplaces and plugins to ~/.claude/settings.json, keeping
// everything else there, then has the claude CLI add each marketplace and
// install each plugin. Both are no-ops when already done. Its last line
// is JSON: no_cli when there's no claude to run, and the plugins that
// failed.
// Plugins that install by running a command need a person to accept the
// command, so they fail here rather than run unseen.
const claudeScript = `import base64, json, os, shutil, subprocess, sys

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
    sys.exit(path + " isn't valid JSON")
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

cli = shutil.which("claude")
if not cli:
    print(json.dumps({"no_cli": True}))
    sys.exit(0)

def run(*args):
    try:
        p = subprocess.run([cli, "plugin", *args, "--json"], stdin=subprocess.DEVNULL,
                           capture_output=True, text=True, timeout=600)
    except subprocess.TimeoutExpired:
        return "timed out"
    if p.returncode == 0:
        return None
    for line in reversed(p.stdout.splitlines()):
        try:
            return json.loads(line).get("message") or "failed"
        except ValueError:
            pass
    return (p.stderr.strip().splitlines() or ["exit status %d" % p.returncode])[-1]

failed = []
broken = {}
for name, src in data["marketplaces"].items():
    err = run("marketplace", "add", src)
    if err:
        broken[name] = err
for plugin in data["plugins"]:
    err = broken.get(plugin.split("@", 1)[1]) or run("install", plugin)
    if err:
        failed.append(plugin + ": " + err)
print(json.dumps({"failed": failed}))
`
