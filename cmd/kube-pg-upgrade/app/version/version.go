package version

import (
	"github.com/spf13/cobra"

	versionpkg "github.com/containerinfra/kube-pg-upgrade/pkg/version"
)

var (
	version string
	commit  string
	branch  string
)

// NewVersionCmd returns the Cobra version sub command
func NewVersionCmd() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `version information`,
		Run: func(cmd *cobra.Command, args []string) {
			versionpkg.Print()
		},
	}

	return versionCmd
}
