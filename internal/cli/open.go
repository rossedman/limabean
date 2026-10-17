package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newOpenCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "open [name] [port]",
		Short: "Open a port a VM forwards in your browser",
		Long: `Open http://localhost:<port> in your default browser, for a dev server or
container running in a VM (see "limabean ports").

Leave out the port and the VM's only forwarded port is opened, or you pick
one from a list. Leave out both to pick the VM too. A single number is
taken as the port when it isn't a VM's name.`,
		Example: `  limabean open dev 3000

  # dev's only forwarded port, or pick one
  limabean open dev

  # Port 8080 of the only VM, or pick the VM
  limabean open 8080`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: a.completeVMs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			var name, port string
			switch len(args) {
			case 2:
				name, port = args[0], args[1]
			case 1:
				if _, ok, err := a.Lima.Get(ctx, args[0]); err != nil {
					return err
				} else if ok {
					name = args[0]
				} else if _, err := strconv.Atoi(args[0]); err == nil {
					port = args[0]
				} else {
					return fmt.Errorf("no VM named %q (see: limabean ls)", args[0])
				}
			}
			if name == "" {
				picked, err := a.vmArg(ctx, nil, "open a port of")
				if err != nil {
					return err
				}
				name = picked
			}
			inst, err := a.vm(ctx, name)
			if err != nil {
				return err
			}
			if !inst.Running() {
				return fmt.Errorf("%s is stopped, so nothing is forwarded (start it with: limabean start %s)", name, name)
			}

			if port != "" {
				n, err := strconv.Atoi(port)
				if err != nil || n < 1 || n > 65535 {
					return fmt.Errorf("%q isn't a port number", port)
				}
				if !a.macListens(n) {
					a.UI.Warn("Nothing answers on localhost:%d yet; opening it anyway", n)
					a.UI.Hint("See what's forwarded with: limabean ports " + name)
				}
				return a.open(ctx, fmt.Sprintf("http://localhost:%d", n))
			}

			list, err := a.ports(ctx, inst)
			if err != nil {
				return err
			}
			var urls []string
			for _, p := range list {
				if p.Forwarded {
					urls = append(urls, p.URL)
				}
			}
			switch {
			case len(urls) == 0:
				return fmt.Errorf("nothing in %s is forwarded to your Mac yet (see: limabean ports %s)", name, name)
			case len(urls) == 1:
				return a.open(ctx, urls[0])
			case !a.UI.Prompting:
				return errors.New("several ports are forwarded; name one (see: limabean ports " + name + ")")
			}
			url, err := a.UI.Select("Port to open", urls)
			if err != nil {
				return err
			}
			return a.open(ctx, url)
		},
	}
}

// open opens a URL in the browser, saying which.
func (a *App) open(ctx context.Context, url string) error {
	a.UI.Info("Opening %s", a.UI.Code(url))
	return a.openURL(ctx, url)
}
