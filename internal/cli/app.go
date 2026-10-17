// Package cli is limabean's command line, built with Cobra and Viper.
//
// Each command lives in its own file, in a constructor that takes the
// shared *App: root.go wires them together, and groups (claude, github,
// secrets) have a file for the parent command and one per subcommand.
// Helpers shared by several commands live in vm.go and secrets_flow.go.
package cli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/rossedman/limabean/internal/claude"
	"github.com/rossedman/limabean/internal/config"
	"github.com/rossedman/limabean/internal/github"
	"github.com/rossedman/limabean/internal/keychain"
	"github.com/rossedman/limabean/internal/lima"
	"github.com/rossedman/limabean/internal/secrets"
	"github.com/rossedman/limabean/internal/templates"
	"github.com/rossedman/limabean/internal/ui"
)

// Keychain is the part of the macOS Keychain limabean uses.
type Keychain interface {
	Get(service string) (string, error)
	Set(service, value string) error
	Delete(service string) error
}

// App is what every command shares. Its fields are set up in the root
// command's PersistentPreRunE, after flags are parsed.
type App struct {
	Version string

	V        *viper.Viper
	Cfg      *config.Config
	UI       *ui.UI
	Lima     *lima.Client
	Claude   claude.Settings
	Keychain Keychain
	GitHub   github.Client
	Resolver secrets.Resolver

	// Embedded templates; tests can replace them.
	TemplatesFS fs.FS

	// How `ssh` and `get` reach a VM's terminal; tests replace them.
	execSSH   func(inst lima.Instance, command ...string) error
	attachSSH func(inst lima.Instance, command ...string) error
	// How `autostop` drives launchd; tests replace it.
	launchctl func(ctx context.Context, args ...string) ([]byte, error)
	// How `code` starts the editor; tests replace it.
	openEditor func(ctx context.Context, ide string, args ...string) error
	// How snapshots copy disks; tests replace it.
	cloneFile func(ctx context.Context, src, dst string) error
	// How `ports` and `open` reach the Mac's side; tests replace them.
	macListens func(port int) bool
	openURL    func(ctx context.Context, url string) error

	// Persistent flags.
	configPath   string
	plain        bool
	templatesDir string

	// Overridable for tests.
	stateDir       string
	keychainPrefix string
}

// NewApp returns an App wired to the real world.
func NewApp(version string, embedded fs.FS) *App {
	return &App{
		Version:     version,
		V:           config.New(),
		TemplatesFS: embedded,
		execSSH:     lima.Exec,
		attachSSH:   attach,
		launchctl:   runLaunchctl,
		openEditor:  openEditor,
		cloneFile:   cloneFile,
		macListens:  macListens,
		openURL:     openURL,
	}
}

// setup loads the config and builds the clients. Fields already set (by
// tests) are kept.
func (a *App) setup() error {
	cfg, err := config.Load(a.V, a.configPath)
	if err != nil {
		return err
	}
	a.Cfg = cfg
	if a.UI == nil {
		a.UI = ui.New(a.plain || a.V.GetBool(config.KeyPlain))
	}
	if a.Lima == nil {
		a.Lima = lima.New()
	}
	if a.Claude.Path == "" {
		a.Claude.Path = envOr("LIMABEAN_CLAUDE_SETTINGS", claude.DefaultPath())
	}
	if a.Keychain == nil {
		a.Keychain = keychain.Default()
	}
	if a.Resolver.Keychain == nil {
		a.Resolver = secrets.Resolver{
			Keychain:   a.Keychain.Get,
			LookupEnv:  os.LookupEnv,
			Run:        secrets.RunCommand,
			HasProgram: hasProgram,
		}
	}
	if a.stateDir == "" {
		a.stateDir = stateDir()
	}
	if a.keychainPrefix == "" {
		a.keychainPrefix = envOr("LIMABEAN_KEYCHAIN_PREFIX", "limabean")
	}
	return nil
}

// templatesRoot is the directory limactl reads templates from.
func (a *App) templatesRoot() (string, error) {
	return templates.Dirs{
		Override: firstNonEmpty(a.templatesDir, a.V.GetString(config.KeyTemplates)),
		User:     filepath.Join(a.Cfg.Dir(), "templates"),
		Cache:    filepath.Join(cacheDir(), "templates"),
	}.Root(a.TemplatesFS)
}

// githubService is the Keychain item holding the GitHub token.
func (a *App) githubService() string { return a.keychainPrefix + "-github-token" }

// githubRef is the secrets reference for the GitHub token.
func (a *App) githubRef() string { return "keychain:" + a.githubService() }

// secretService is the Keychain item for a value saved by limabean: shared
// (vm == "") or for one VM.
func (a *App) secretService(vm, name string) string {
	if vm == "" {
		return a.keychainPrefix + "-secret-" + name
	}
	return a.keychainPrefix + "-secret-" + vm + "-" + name
}

// ownsKeychainRef reports whether a reference points at a Keychain item
// limabean created, which it may delete along with the reference.
func (a *App) ownsKeychainRef(ref string) bool {
	prefix := "keychain:" + a.keychainPrefix + "-secret-"
	return len(ref) > len(prefix) && ref[:len(prefix)] == prefix
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func stateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "limabean")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "limabean")
}

func cacheDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "limabean")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "limabean")
}

func hasProgram(name string) bool {
	_, err := lookPath(name)
	return err == nil
}
