package postgres

import (
	"github.com/spf13/cobra"
)

// NewPostgresCmd returns cobra.Command to run the kube-pg-upgrade databases postgres subcommand
func NewPostgresCmd() *cobra.Command {
	cmds := &cobra.Command{
		Use:     "postgres",
		Short:   "Perform actions on a PostgreSQL running in Kubernetes",
		Long:    "Perform actions on a PostgreSQL running in Kubernetes",
		Aliases: []string{"pg", "pgsql", "postgresql"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmds.ResetFlags()
	cmds.AddCommand(NewUpgradePostgresCmd(nil))
	return cmds
}
