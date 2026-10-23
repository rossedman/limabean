package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/project"
)

// newInitCmd builds `limabean init`, which sets up limabean or writes a
// repository's .limabean.yaml.
func newInitCmd(a *App) *cobra.Command {
	var forProject, toStdout, overwrite bool
	var dir string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up limabean, or write a repository's .limabean.yaml",
		Long: `Walk through first-time setup, then run "limabean doctor":

  1. Where your git config lives on the Mac. A ~/.gitconfig symlinked into a
     dotfiles repository is followed, and that directory is suggested.
  2. Where repositories go inside VMs (default ~/code).
  3. Whether to add "Include ~/.lima/*/ssh.config" to the top of
     ~/.ssh/config (a backup is kept), so ssh lima-<name> and Claude Desktop
     can reach VMs.
  4. Optionally, a GitHub token for gh and https pushes in VMs.

Settings are saved to ~/.config/limabean/config. Without a terminal, the
suggested values are saved and the SSH config isn't changed.

With --project, init writes a .limabean.yaml for the repository you're in
instead, from what's in it: go.mod, package.json and its lockfile,
pyproject.toml, Cargo.toml, Gemfile, mix.exs, pom.xml, a compose file,
.tool-versions and the like, here and in the directories directly below. It
picks the packs, the setup commands, likely ports, and tools no pack has,
and lists credentials named in .env.example as secrets to set up. It only
reads files; nothing in the repository runs. In a terminal it shows what it
found and asks before writing. --print writes the file to standard output
instead, --overwrite replaces a project file that's already there, and
--dir looks at another directory than the current one.`,
		Example: `  limabean init

  # Accept every suggestion, e.g. in a provisioning script
  limabean init --plain

  # A project file for this repository, then its VM
  cd ~/src/myapp && limabean init --project && limabean up

  # See what it would write, for another checkout
  limabean init --project --print --dir ~/src/api`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if forProject {
				return a.initProject(dir, toStdout, overwrite)
			}
			if toStdout || overwrite || dir != "" {
				return usageErr("--print, --overwrite and --dir go with --project")
			}
			home, _ := os.UserHomeDir()
			cfg := *a.Cfg
			cfg.GitConfigDir = guessGitConfigDir(home, cfg.GitConfigDir)
			addInclude, addToken := true, false
			sshConfig := filepath.Join(home, ".ssh", "config")
			inc, _ := sshInclude(sshConfig)
			_, tokenErr := a.Keychain.Get(a.githubService())

			if a.UI.Prompting {
				fields := []huh.Field{
					huh.NewNote().Title("limabean setup").
						Description("Saved to " + shortPath(cfg.Path) + ". Press enter to keep a suggestion."),
					huh.NewInput().Title("Host directory with your git config").
						Description("Mounted read-only into every VM.").Value(&cfg.GitConfigDir),
					huh.NewInput().Title("Where repositories go inside VMs").
						Description("ghq clones to <dir>/<host>/<owner>/<repo>.").Value(&cfg.CodeDir),
				}
				if inc == 0 {
					fields = append(fields, huh.NewConfirm().
						Title("Add 'Include ~/.lima/*/ssh.config' to the top of ~/.ssh/config?").
						Description("Needed for ssh lima-<name> and Claude Desktop. A backup is kept.").
						Value(&addInclude))
				}
				if tokenErr != nil {
					fields = append(fields, huh.NewConfirm().
						Title("Save a GitHub token for gh and https pushes in VMs now?").
						Description("You can do it later with: limabean github login").
						Value(&addToken))
				}
				if err := a.UI.Run(huh.NewGroup(fields...)); err != nil {
					return err
				}
			} else {
				addInclude = false
			}

			cfg.GitConfigDir = strings.TrimSpace(cfg.GitConfigDir)
			cfg.CodeDir = strings.TrimSpace(cfg.CodeDir)
			if err := cfg.Save(); err != nil {
				return err
			}
			*a.Cfg = cfg
			a.UI.OK("Saved %s", shortPath(cfg.Path))

			if inc == 0 {
				if addInclude {
					if err := addSSHInclude(sshConfig); err != nil {
						return err
					}
					a.UI.OK("Updated ~/.ssh/config (backup: ~/.ssh/config.limabean.bak)")
				} else {
					a.UI.Warn("SSH config not changed; ssh lima-<name> needs the Include line")
				}
			}
			if addToken {
				login := newGitHubLoginCmd(a)
				login.SetContext(cmd.Context())
				if err := login.RunE(login, nil); err != nil && !errors.Is(err, errSilent) {
					return err
				}
			}
			a.doctor(cmd.Context())
			if _, err := os.Stat(".git"); err == nil {
				if _, err := project.Find("."); err != nil {
					a.UI.Hint("This is a repository without a project file. Write one: limabean init --project")
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&forProject, "project", false, "write a .limabean.yaml for the repository here instead")
	cmd.Flags().BoolVar(&toStdout, "print", false, "with --project, write it to standard output")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "with --project, replace a project file that's already here")
	cmd.Flags().StringVar(&dir, "dir", "", "with --project, the repository to look at, instead of the current directory")
	return cmd
}

// guessGitConfigDir suggests where the git config lives: the directory a
// symlinked ~/.gitconfig points into, else the current setting.
func guessGitConfigDir(home, current string) string {
	target, err := filepath.EvalSymlinks(filepath.Join(home, ".gitconfig"))
	if err != nil {
		return current
	}
	if fi, err := os.Lstat(filepath.Join(home, ".gitconfig")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return current
	}
	dir := filepath.Dir(target)
	// Resolve home too, so a symlinked home (or /var → /private/var on
	// macOS) still compares equal to the resolved target.
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
		return "~/" + rel
	}
	return dir
}

// addSSHInclude puts Lima's Include at the top of the ssh config, keeping a
// backup.
func addSSHInclude(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err := os.WriteFile(path+".limabean.bak", old, 0o600); err != nil {
			return err
		}
	}
	return os.WriteFile(path, append([]byte("Include ~/.lima/*/ssh.config\n\n"), old...), 0o600)
}
