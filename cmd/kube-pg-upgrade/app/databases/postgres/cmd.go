package postgres

import (
	"github.com/spf13/cobra"
)

// NewPostgresCmd returns cobra.Command to run the kube-pg-upgrade databases postgres subcommand
func NewPostgresCmd() *cobra.Command {
	cmds := &cobra.Command{
		Use:     "pgupgrade",
		Short:   "Perform a PG upgrade PostgreSQL running in Kubernetes",
		Long:    "Perform a PG upgrade PostgreSQL running in Kubernetes",
		Aliases: []string{"pg_upgrade", "pg-upgrade", "upgrade"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmds.ResetFlags()
	cmds.AddCommand(NewUpgradePostgresStatefulSetCmd(nil))
	cmds.AddCommand(NewUpgradePostgresPVCCmd(nil))
	return cmds
}
