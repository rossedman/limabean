package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/rossedman/limabean/internal/secrets"
)

// Where a secret's value comes from, as offered in forms.
const (
	sourcePaste    = "paste"
	sourceOnePass  = "op"
	sourceEnv      = "env"
	sourceCommand  = "cmd"
	sourceExisting = "ref"
)

// newSecret is a secret being added.
type newSecret struct {
	Name   string
	Source string // one of the source* constants
	Input  string // the pasted value, or the rest of the reference
	Shared bool   // every VM, rather than the one being created
}

// addSecretGroups returns form groups that fill s: the name and source,
// then the value (hidden as you type) or the reference, then, when vm is
// set, whether the secret is for every VM or just that one.
func addSecretGroups(s *newSecret, vm string) []*huh.Group {
	notEmpty := func(v string) error {
		if strings.TrimSpace(v) == "" {
			return errors.New("required")
		}
		return nil
	}
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().Title("Name").Description("The environment variable, e.g. OPENAI_API_KEY").
				Placeholder("OPENAI_API_KEY").Value(&s.Name).
				Validate(func(v string) error {
					if !secrets.ValidName(strings.TrimSpace(v)) {
						return errors.New("letters, digits and _, not starting with a digit")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Where does the value come from?").
				Options(
					huh.NewOption("Paste it now (saved in your Keychain)", sourcePaste),
					huh.NewOption("1Password (op://vault/item/field)", sourceOnePass),
					huh.NewOption("An environment variable on this Mac", sourceEnv),
					huh.NewOption("A command's output, run on this Mac", sourceCommand),
				).Value(&s.Source),
		),
		huh.NewGroup(
			huh.NewInput().Title("Value").Description("Saved in your macOS Keychain, not in a file.").
				EchoMode(huh.EchoModePassword).Value(&s.Input).Validate(notEmpty),
		).WithHideFunc(func() bool { return s.Source != sourcePaste }),
		huh.NewGroup(
			huh.NewInput().
				TitleFunc(func() string {
					switch s.Source {
					case sourceOnePass:
						return "1Password reference"
					case sourceEnv:
						return "Variable name on this Mac"
					}
					return "Command"
				}, &s.Source).
				DescriptionFunc(func() string {
					switch s.Source {
					case sourceOnePass:
						return "Read with the op CLI each time secrets are synced."
					case sourceEnv:
						return "Read from the shell you run limabean in."
					}
					return "Run with sh on this Mac each time secrets are synced."
				}, &s.Source).
				PlaceholderFunc(func() string {
					switch s.Source {
					case sourceOnePass:
						return "op://Private/OpenAI/credential"
					case sourceEnv:
						return "OPENAI_API_KEY"
					}
					return "pass show openai"
				}, &s.Source).
				Value(&s.Input).
				Validate(func(v string) error {
					v = strings.TrimSpace(v)
					switch {
					case v == "":
						return errors.New("required")
					case s.Source == sourceOnePass && !strings.HasPrefix(v, "op://"):
						return errors.New("starts with op://")
					case s.Source == sourceEnv && !secrets.ValidName(v):
						return errors.New("a variable name")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return s.Source == sourcePaste }),
	}
	if vm != "" {
		groups = append(groups, huh.NewGroup(huh.NewConfirm().
			Title("Use it in every VM?").
			Description(fmt.Sprintf("No keeps it to %s.", vm)).
			Affirmative("Every VM").Negative("Just "+vm).
			Value(&s.Shared)))
	}
	return groups
}

// saveSecret stores a new secret: a pasted value goes into the Keychain,
// anything else is written as a reference. vm is the VM it's for, or "".
func (a *App) saveSecret(s newSecret, vm string) (file string, err error) {
	name := strings.TrimSpace(s.Name)
	input := strings.TrimSpace(s.Input)
	scope := vm
	if s.Shared {
		scope = ""
	}
	var ref string
	switch s.Source {
	case sourcePaste:
		service := a.secretService(scope, name)
		if err := a.Keychain.Set(service, input); err != nil {
			return "", fmt.Errorf("saving %s to the Keychain: %w", name, err)
		}
		ref = "keychain:" + service
	case sourceOnePass, sourceExisting:
		ref = input
	case sourceEnv:
		ref = "env:" + input
	case sourceCommand:
		ref = "cmd:" + input
	default:
		return "", fmt.Errorf("unknown source %q", s.Source)
	}
	file = secrets.SharedFile(a.Cfg.Dir())
	if scope != "" {
		file = secrets.VMFile(a.Cfg.Dir(), scope)
	}
	return file, secrets.Put(file, name, ref)
}

// otherVMSecrets lists secrets in other VMs' files that this VM doesn't get,
// for offering when a VM is created: name → reference (first file wins).
func (a *App) otherVMSecrets(vm string) (names []string, refs map[string]secrets.Secret) {
	refs = map[string]secrets.Secret{}
	shared, _ := secrets.Load(a.Cfg.Dir(), "")
	inShared := map[string]bool{}
	for _, s := range shared {
		inShared[s.Name] = true
	}
	files, _ := filepath.Glob(filepath.Join(a.Cfg.Dir(), "secrets.*.env"))
	slices.Sort(files)
	for _, f := range files {
		if filepath.Base(f) == "secrets."+vm+".env" {
			continue
		}
		list, _ := secrets.LoadFile(f)
		for _, s := range list {
			if inShared[s.Name] {
				continue
			}
			if _, seen := refs[s.Name]; !seen {
				refs[s.Name] = s
				names = append(names, s.Name)
			}
		}
	}
	slices.Sort(names)
	return names, refs
}
