package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"
)

func newAutostopRunCmd(a *App) *cobra.Command {
	var idle time.Duration
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Stop idle VMs and delete expired ones, once",
		Long: `Do what the autostop agent does every 10 minutes, once: stop every VM
nobody has used for --idle (as "limabean stop --idle"), and delete every VM
made with "new --ttl" that has expired (as "limabean rm --expired").`,
		Example: `  limabean autostop run --idle 2h`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if idle < time.Minute {
				return errors.New("--idle needs a duration of a minute or more, e.g. 2h")
			}
			return errors.Join(a.removeExpired(ctx), a.stopIdle(ctx, nil, idle, false))
		},
	}
	cmd.Flags().DurationVar(&idle, "idle", 2*time.Hour, "stop VMs unused for this long")
	return cmd
}
