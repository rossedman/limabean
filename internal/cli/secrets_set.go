package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/secrets"
)

func newSecretsSetCmd(a *App) *cobra.Command {
	var vm string
	var fromStdin, noSync bool
	cmd := &cobra.Command{
		Use:   "set [NAME] [reference]",
		Short: "Add or change a secret",
		Long: `Add a secret, or change where an existing one comes from, then install it in
running VMs.

With a reference (keychain:…, op://…, env:…, cmd:… or a plain value), it's
written as given. Without one, you're asked for the value, which is saved in
your Keychain as limabean-secret-NAME (limabean-secret-<vm>-NAME with --vm)
and referenced from there; --from-stdin reads the value from stdin instead.
With no arguments at all, a form asks for everything.

The secret goes to every VM, or with --vm to that VM only.`,
		Example: `  # Ask for everything
  limabean secrets set

  # Paste the value at a hidden prompt; saved in the Keychain
  limabean secrets set OPENAI_API_KEY

  # From a password manager or script, without a prompt
  op read op://Private/OpenAI/credential | limabean secrets set OPENAI_API_KEY --from-stdin

  # References: 1Password, an environment variable, a command
  limabean secrets set ANTHROPIC_API_KEY op://Private/Anthropic/credential
  limabean secrets set NPM_TOKEN env:NPM_TOKEN --vm web
  limabean secrets set VAULT_TOKEN 'cmd:vault print token'

  # A plain setting
  limabean secrets set AWS_REGION us-east-1`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if vm != "" {
				if err := validVMName(vm); err != nil {
					return fmt.Errorf("--vm %q: %w", vm, err)
				}
			}
			var file string
			switch {
			case len(args) == 0:
				if !a.UI.Prompting {
					return errors.New("name the secret: limabean secrets set NAME [reference]")
				}
				s := newSecret{Source: sourcePaste}
				if err := a.UI.Run(addSecretGroups(&s, vm)...); err != nil {
					return err
				}
				f, err := a.saveSecret(s, vm)
				if err != nil {
					return err
				}
				file = f
				args = []string{strings.TrimSpace(s.Name)}
			case len(args) == 2:
				if !secrets.ValidName(args[0]) {
					return fmt.Errorf("%q isn't a valid variable name", args[0])
				}
				a.warnPlainSecret(args[0], args[1])
				f, err := a.saveSecret(newSecret{Name: args[0], Source: sourceExisting, Input: args[1], Shared: vm == ""}, vm)
				if err != nil {
					return err
				}
				file = f
			default:
				if !secrets.ValidName(args[0]) {
					return fmt.Errorf("%q isn't a valid variable name", args[0])
				}
				var value string
				var err error
				if fromStdin || !a.UI.Prompting {
					value, err = readLine(cmd)
				} else {
					value, err = a.UI.Password(args[0], "hidden as you type; saved in your Keychain")
				}
				if err != nil {
					return err
				}
				if value == "" {
					return errors.New("no value given")
				}
				f, err := a.saveSecret(newSecret{Name: args[0], Source: sourcePaste, Input: value, Shared: vm == ""}, vm)
				if err != nil {
					return err
				}
				file = f
			}
			a.UI.OK("%s → %s", args[0], shortPath(file))
			if noSync {
				return nil
			}
			return a.syncSecrets(ctx, "")
		},
	}
	f := cmd.Flags()
	f.StringVar(&vm, "vm", "", "only for this VM (default: every VM)")
	f.BoolVar(&fromStdin, "from-stdin", false, "read the value from stdin instead of prompting")
	f.BoolVar(&noSync, "no-sync", false, "don't install it in running VMs yet")
	return cmd
}

// readLine reads one line from the command's stdin, without its newline.
func readLine(cmd *cobra.Command) (string, error) {
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("nothing on stdin")
	}
	return strings.TrimRight(line, "\r\n"), nil
}
