package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/ui"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/secrets"
)

// newDoctorCmd builds `limabean doctor`, which checks the things VMs depend
// on.
func newDoctorCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check everything VMs depend on",
		Long: `Check your Mac for everything limabean and its VMs rely on, and print the
exact fix for anything missing:

  Lima          limactl 2.0 or newer
  SSH config    Lima's host entries are included at the top of ~/.ssh/config
  SSH agent     a key is loaded, so VMs can push with it
  Git config    the directory mounted into VMs has a git config
  Claude        Desktop's settings file can be updated
  GitHub        the saved token works, and when it expires
  Secrets       every reference can be looked up (values aren't read)
  AWS           whether ~/.aws will be mounted
  Project       in a checkout with a .limabean.yaml: the file loads, and its
                packs, network presets and secrets exist on this Mac; and
                whether its VM matches it

It exits non-zero if something needs fixing, so it can gate scripts.`,
		Example: `  limabean doctor

  # In a script
  limabean doctor --plain >/dev/null || echo "fix limabean first"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// The report is the result, so it goes to stdout.
			defer func(u *ui.UI) { a.UI = u }(a.UI)
			a.UI = a.UI.Report()
			if !a.doctor(cmd.Context()) {
				a.UI.Box("Fix the items marked ✗ above.")
				return errSilent
			}
			a.UI.Box(a.UI.Code("All good.") + " Create a VM with: limabean up <name>")
			return nil
		},
	}
}

// doctor runs every check and reports whether all passed.
func (a *App) doctor(ctx context.Context) bool {
	ok := true
	fail := func(msg string, hints ...string) { a.UI.Bad("%s", msg); a.UI.Hint(hints...); ok = false }

	a.UI.Section("Lima", "")
	if v, err := a.Lima.Version(ctx); err != nil {
		fail("limactl not found", "brew install lima")
	} else if major, _ := strconv.Atoi(strings.SplitN(v, ".", 2)[0]); major < 2 {
		fail(fmt.Sprintf("Lima %s is too old; limabean needs 2.0 or newer", v), "brew upgrade lima")
	} else {
		a.UI.OK("Lima %s", v)
	}

	a.UI.Section("SSH config", "~/.ssh/config")
	home, _ := os.UserHomeDir()
	switch inc, block := sshInclude(filepath.Join(home, ".ssh", "config")); {
	case inc == 0:
		fail("doesn't include Lima's host entries, so ssh lima-<name> and Claude Desktop can't reach VMs",
			"Add this line at the top of ~/.ssh/config, or run limabean init:", "  Include ~/.lima/*/ssh.config")
	case block > 0 && block < inc:
		fail(fmt.Sprintf("the Include on line %d sits inside the Host/Match block on line %d, so it doesn't apply", inc, block),
			"Move it to the top of ~/.ssh/config.")
	default:
		a.UI.OK("Include ~/.lima/*/ssh.config (line %d)", inc)
	}

	a.UI.Section("SSH agent", "lets VMs push with your keys")
	out, err := exec.CommandContext(ctx, "ssh-add", "-l").Output()
	var exit *exec.ExitError
	switch {
	case err == nil:
		a.UI.OK("%d key(s) loaded", strings.Count(strings.TrimSpace(string(out)), "\n")+1)
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		fail("the agent has no keys, so git push over SSH will fail",
			"ssh-add --apple-use-keychain ~/.ssh/id_ed25519   (or your key)",
			"To keep it after a reboot, add under 'Host *' in ~/.ssh/config:", "  AddKeysToAgent yes", "  UseKeychain yes")
	default:
		fail(fmt.Sprintf("no ssh-agent reachable (SSH_AUTH_SOCK=%s)", firstNonEmpty(os.Getenv("SSH_AUTH_SOCK"), "unset")))
	}

	a.UI.Section("Git config", a.Cfg.GitConfigDir)
	dir := config.Expand(a.Cfg.GitConfigDir)
	found := ""
	for _, f := range []string{"config", ".gitconfig"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			found = shortPath(filepath.Join(dir, f))
			break
		}
	}
	if found == "" {
		fail("no config or .gitconfig there; VMs won't know your git identity",
			"Run limabean init, or move ~/.gitconfig to ~/.config/git/config.")
	} else {
		a.UI.OK("found %s", found)
	}

	a.UI.Section("Claude Desktop", "each VM as an SSH connection")
	if !a.Cfg.ClaudeDesktop {
		a.UI.OK("off (CLAUDE_DESKTOP in the config)")
	} else if names, err := a.Claude.VMs(); err != nil {
		fail(err.Error(), "Fix "+shortPath(a.Claude.Path)+", then: limabean claude sync")
	} else {
		a.UI.OK("%d VM(s) registered in %s", len(names), shortPath(a.Claude.Path))
	}

	a.UI.Section("GitHub token", "optional: gh, the API and https pushes in VMs")
	if token, err := a.Keychain.Get(a.githubService()); err != nil {
		a.UI.Warn("none saved")
		a.UI.Hint("limabean github login")
	} else if info, err := a.GitHub.Check(ctx, token); err != nil {
		fail(fmt.Sprintf("the saved token doesn't work: %v", err), "limabean github login")
	} else {
		a.UI.OK("token for %s, expires: %s", info.Login, info.Expires)
	}

	a.UI.Section("Secrets", shortPath(a.Cfg.Dir()))
	if !a.doctorSecrets() {
		ok = false
	}

	a.UI.Section("AWS", "optional: the aws CLI in VMs")
	switch fi, err := os.Stat(config.Expand(a.Cfg.AWSDir)); {
	case a.Cfg.AWSDir == "":
		a.UI.OK("off (AWS_DIR is empty)")
	case err == nil && fi.IsDir():
		a.UI.OK("%s is mounted read-only in new VMs", a.Cfg.AWSDir)
	default:
		a.UI.Hint(a.Cfg.AWSDir + " doesn't exist, so nothing is mounted")
	}

	if !a.doctorProject(ctx) {
		ok = false
	}

	a.UI.Section("Config", shortPath(a.Cfg.Path))
	if _, err := os.Stat(a.Cfg.Path); err == nil {
		a.UI.OK("found")
	} else {
		a.UI.Warn("not created yet; using defaults")
		a.UI.Hint("limabean init")
	}
	return ok
}

// doctorSecrets checks every reference can be looked up, without reading
// values (so 1Password never prompts).
func (a *App) doctorSecrets() bool {
	ok := true
	files, _ := filepath.Glob(filepath.Join(a.Cfg.Dir(), "secrets*.env"))
	total := 0
	for _, f := range files {
		list, errs := secrets.LoadFile(f)
		for _, e := range errs {
			a.UI.Bad("%v", e)
			ok = false
		}
		total += len(list)
		for _, s := range list {
			where := fmt.Sprintf("%s (%s)", s.Name, filepath.Base(f))
			switch {
			case strings.HasPrefix(s.Ref, "keychain:"):
				if _, err := a.Keychain.Get(strings.TrimPrefix(s.Ref, "keychain:")); err != nil {
					a.UI.Bad("%s: no Keychain item %s", where, strings.TrimPrefix(s.Ref, "keychain:"))
					a.UI.Hint("limabean secrets set " + s.Name)
					ok = false
				}
			case strings.HasPrefix(s.Ref, "op://"):
				if !hasProgram("op") {
					a.UI.Bad("%s: needs the 1Password CLI", where)
					a.UI.Hint("brew install 1password-cli")
					ok = false
				}
			case strings.HasPrefix(s.Ref, "env:"):
				if _, set := os.LookupEnv(strings.TrimPrefix(s.Ref, "env:")); !set {
					a.UI.Warn("%s: $%s isn't set in this shell", where, strings.TrimPrefix(s.Ref, "env:"))
				}
			}
		}
	}
	if total == 0 {
		a.UI.Hint("none yet (limabean secrets set NAME)")
	} else if ok {
		a.UI.OK("%d secret(s) in %d file(s), all resolvable", total, len(files))
	}
	return ok
}

var (
	includeRE = regexp.MustCompile(`(?i)^\s*Include\s.*\.lima/\*/ssh\.config`)
	blockRE   = regexp.MustCompile(`(?i)^\s*(Host|Match)\s`)
)

// sshInclude returns the line of Lima's Include and of the first Host or
// Match block in an ssh config (0 when absent).
func sshInclude(path string) (include, block int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if include == 0 && includeRE.MatchString(line) {
			include = n
		}
		if block == 0 && blockRE.MatchString(line) {
			block = n
		}
	}
	return include, block
}
