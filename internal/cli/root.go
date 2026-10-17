package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/ui"
)

// NewRootCmd builds the command tree.
func NewRootCmd(a *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "limabean",
		Short: "Disposable Linux dev VMs on macOS, ready for SSH and Claude",
		Long: `limabean creates Ubuntu dev VMs on your Mac from layered Lima templates.

Every VM gets a version-pinned toolchain (Docker, Go, Python, Kubernetes and
Terraform tooling, and more), your git identity, read-only access to your AWS
profiles, and the secrets you choose. It's reachable as ssh lima-<name>, and
is registered as an SSH connection in Claude Desktop. Your SSH keys stay on
the Mac: git pushes through ssh-agent forwarding, or over https with a
GitHub token kept in your Keychain.

limabean drives limactl, so plain Lima commands keep working alongside it.
In a terminal it shows spinners, pickers and forms; when output isn't a
terminal, or with --plain, it prints plain text and never prompts.`,
		Example: `  # Set up once: settings, SSH config, a GitHub token
  limabean init

  # Create a VM and connect to it
  limabean new dev
  limabean ssh dev

  # Give every VM an API key from 1Password
  limabean secrets set OPENAI_API_KEY op://Private/OpenAI/credential

  # See what you have
  limabean ls`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       a.Version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return a.setup()
		},
	}
	root.SetVersionTemplate("limabean {{.Version}}\n")
	setupHelp(root, a)
	root.CompletionOptions.DisableDefaultCmd = true

	pf := root.PersistentFlags()
	pf.StringVar(&a.configPath, "config", "", "config file (default ~/.config/limabean/config)")
	pf.BoolVar(&a.plain, "plain", false, "print plain text and never prompt (also LIMABEAN_PLAIN=1)")
	pf.StringVar(&a.templatesDir, "templates", "", "read templates from this directory instead of the built-in copy (also LIMABEAN_TEMPLATES)")
	_ = a.V.BindPFlag(config.KeyPlain, pf.Lookup("plain"))
	_ = a.V.BindPFlag(config.KeyTemplates, pf.Lookup("templates"))

	root.AddGroup(
		&cobra.Group{ID: "setup", Title: "Setup:"},
		&cobra.Group{ID: "vms", Title: "VMs:"},
		&cobra.Group{ID: "access", Title: "Secrets and access:"},
	)
	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	add("setup", newInitCmd(a), newDoctorCmd(a))
	add("vms", newNewCmd(a), newLsCmd(a), newSSHCmd(a), newGetCmd(a), newCodeCmd(a), newPortsCmd(a), newOpenCmd(a), newStartCmd(a), newStopCmd(a), newRebuildCmd(a), newRmCmd(a), newSnapshotCmd(a), newAutostopCmd(a))
	add("access", newSecretsCmd(a), newGitHubCmd(a), newNetworkCmd(a), newClaudeCmd(a), newMCPCmd(a))
	root.AddCommand(newCompletionCmd(a), newVersionCmd(a))
	return root
}

// Execute runs limabean with args (os.Args[1:]) and returns the process exit
// code.
func Execute(version string, embedded fs.FS, args []string) int {
	a := NewApp(version, embedded)
	root := NewRootCmd(a)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ui.ErrAborted):
		return 130
	case errors.Is(err, errSilent):
		return 1
	}
	u := a.UI
	if u == nil {
		u = ui.New(true)
	}
	fmt.Fprintf(os.Stderr, "%s %v\n", u.Dimmed("limabean:"), err)
	return 1
}

// errSilent fails a command whose problems were already printed.
var errSilent = errors.New("failed")
