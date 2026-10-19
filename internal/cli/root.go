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
  limabean up dev
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
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			a.updateNotice(cmd)
		},
	}
	root.SetVersionTemplate("limabean {{.Version}}\n")
	setupHelp(root, a)
	root.SetHelpCommand(newHelpCmd(a))
	root.CompletionOptions.DisableDefaultCmd = true

	pf := root.PersistentFlags()
	pf.StringVar(&a.configPath, "config", "", "config file (default ~/.config/limabean/config)")
	pf.BoolVar(&a.plain, "plain", false, "print plain text and never prompt (also LIMABEAN_PLAIN=1)")
	pf.BoolVar(&a.noColor, "no-color", false, "print without color (also NO_COLOR=1)")
	pf.BoolVarP(&a.quiet, "quiet", "q", false, "print only errors (and the data a command returns)")
	pf.BoolVarP(&a.verbose, "verbose", "v", false, "also print the limactl and ssh commands limabean runs; wins over --quiet")
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
	add("setup", newInitCmd(a), newConfigCmd(a), newDoctorCmd(a), newBumpCmd(a))
	add("vms", newNewCmd(a), newUpCmd(a), newDownCmd(a), newLogsCmd(a), newPsCmd(a), newInfoCmd(a), newRecipesCmd(a), newRunCmd(a), newLsCmd(a), newSSHCmd(a), newGetCmd(a), newCpCmd(a), newCodeCmd(a), newRemoteCmd(a), newPortsCmd(a), newOpenCmd(a), newPacksCmd(a), newStartCmd(a), newStopCmd(a), newRebuildCmd(a), newCloneCmd(a), newRmCmd(a), newSnapshotCmd(a), newImagesCmd(a), newAutostopCmd(a))
	add("access", newSecretsCmd(a), newGitHubCmd(a), newNetworkCmd(a), newClaudeCmd(a), newMCPCmd(a))
	root.AddCommand(newCompletionCmd(a), newVersionCmd(a), newUpdateCheckCmd(a))
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &cliError{code: exitUsage, kind: "usage", err: err}
	})
	usageArgs(root)
	return root
}

// Execute runs limabean with args (os.Args[1:]) and returns the process exit
// code.
func Execute(version string, embedded fs.FS, args []string) (status int) {
	defer func() {
		if r := recover(); r != nil {
			bugReport(os.Stderr, version, args, r)
			status = exitError
		}
	}()
	a := NewApp(version, embedded)
	root := NewRootCmd(a)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, ui.ErrAborted):
		return 130
	case errors.Is(err, errSilent):
		return exitError
	}
	var code exitCode
	if errors.As(err, &code) {
		return int(code)
	}
	status, _ = classify(err)
	var ce *cliError
	if errors.As(err, &ce) && ce.silent {
		return status
	}
	if wantsJSON(args) {
		writeJSONError(os.Stderr, err)
		return status
	}
	u := a.UI
	if u == nil {
		u = ui.New(true, false)
	}
	fmt.Fprintf(os.Stderr, "%s %v\n", u.Dimmed("limabean:"), err)
	return status
}

// errSilent fails a command whose problems were already printed.
var errSilent = errors.New("failed")

// exitCode makes limabean exit with a command's own status (run).
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("the command exited with status %d", int(c)) }
