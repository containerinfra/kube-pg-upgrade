package app

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/containerinfra/kube-pg-upgrade/cmd/kube-pg-upgrade/app/databases/postgres"
	"github.com/containerinfra/kube-pg-upgrade/cmd/kube-pg-upgrade/app/docs"
	"github.com/containerinfra/kube-pg-upgrade/cmd/kube-pg-upgrade/app/version"
)

// Execute runs the kube-pg-upgrade application
func Execute() error {
	cmd := NewACloudToolKitCmd(os.Stdin, os.Stdout, os.Stderr)
	return cmd.Execute()
}

// NewACloudToolKitCmd returns cobra.Command to run the kube-pg-upgrade command
func NewACloudToolKitCmd(in io.Reader, out, err io.Writer) *cobra.Command {
	cmds := &cobra.Command{
		Use:   "kube-pg-upgrade",
		Short: "kube-pg-upgrade for upgrades Postgres on Kubernetes",
		Long:  "kube-pg-upgrade for upgrades Postgres on Kubernetes deployed with the Bitnami or Docker Hub images",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmds.ResetFlags()

	cmds.AddCommand(version.NewVersionCmd())
	cmds.AddCommand(docs.NewOpenDocs())
	cmds.AddCommand(postgres.NewPostgresCmd())

	return cmds
}
