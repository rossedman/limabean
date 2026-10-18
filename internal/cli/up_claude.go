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
// line; Claude Code fetches the plugins in the VM.
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
	if out, err := a.Lima.Run(ctx, inst, bytes.NewReader([]byte(script)), "python3 -"); err != nil {
		a.UI.Warn("Claude plugins not added: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
		return
	}
	a.UI.OK("Claude plugins for %s: %s (installed when a Claude session starts)", inst.Name, strings.Join(c.Plugins(), ", "))
}

// claudeScript runs in the VM with python3, after a line setting DATA. It
// adds the marketplaces and plugins to ~/.claude/settings.json, keeping
// everything else there. Claude Code (the claude pack's, or Claude
// Desktop's over SSH) adds the marketplaces and installs the plugins when
// a session starts.
const claudeScript = `import base64, json, os, sys

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
`
