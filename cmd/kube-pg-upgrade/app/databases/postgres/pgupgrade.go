package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/containerinfra/kube-pg-upgrade/pkg/pgupgrade"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

type postgresVersionInfoOptions struct {
	namespace string

	extraInitDBArgs string
	newPVCDiskSize  string
	timeout         time.Duration

	// nextPostgresVersion is the current postgres version of the database
	// Will attempt auto detection if empty
	currentPostgresVersion string
	// targetPostgresVersion is the postgres version that we will upgrade to
	// Will attempt auto detection if empty
	targetPostgresVersion string

	// username of postgres
	// Will attempt auto detection if empty
	postgresUser string

	targetPVCName string
}

func newPostgresVersionInforOptions() *postgresVersionInfoOptions {
	return &postgresVersionInfoOptions{}
}

func AddPostgresUpgradeFlags(flagSet *flag.FlagSet, opts *postgresVersionInfoOptions) {
	flagSet.StringVarP(&opts.postgresUser, "user", "u", "", "user used for initdb")
	flagSet.StringVarP(&opts.targetPostgresVersion, "target-version", "t", "", "target postgres major version. For example: 14, 15, 16, etc..")
	flagSet.StringVar(&opts.currentPostgresVersion, "current-version", "", "current version of the postgres database. Optional, will attempt auto discovery if left blank. For example: 9.6, 14, 15, 16, etc..")

	flagSet.StringVar(&opts.newPVCDiskSize, "size", "", "New size. Example: 10G")
	flagSet.StringVar(&opts.targetPVCName, "target-pvc-name", "", "Target name of the new pvc. Optional, will use current name by default")
	flagSet.StringVarP(&opts.namespace, "namespace", "n", "", "namespace of the postgres instance. Default is the configured namespace in your kubecontext.")
	flagSet.StringVarP(&opts.extraInitDBArgs, "extra-initdb-args", "i", "", "provide any additional arguments for init-db. Use the same arguments that were provided when the database was originally created. See https://www.postgresql.org/docs/current/pgupgrade.html. Otherwise will attempt to auto detect.")
	// flagSet.StringVarP(&opts.name, "name", "", "name of the statefulset. Required.")
	flagSet.DurationVar(&opts.timeout, "timeout", 0*time.Second, "The length of time to wait before giving up, zero means infinite")
}

//go:embed examples/upgrade.txt
var metadataExamples string

// NewUpgradePostgresCmd returns the Cobra Bootstrap sub command
func NewUpgradePostgresCmd(runOptions *postgresVersionInfoOptions) *cobra.Command {
	if runOptions == nil {
		runOptions = newPostgresVersionInforOptions()
	}

	var cmd = &cobra.Command{
		Use:     "upgrade <statefulset>",
		Args:    cobra.ExactArgs(1),
		Short:   ``,
		Long:    ``,
		Example: metadataExamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			// TODO: configure the timeout
			if runOptions.timeout > 0 {
				timeoutctx, cancelTimeout := context.WithTimeoutCause(ctx, runOptions.timeout, fmt.Errorf("upgrade did not complete within configured timeout (%s)", runOptions.timeout.String()))
				defer cancelTimeout()
				ctx = timeoutctx
			}

			return pgupgrade.RunPGUpgradeForDatabaseStatefulSet(ctx, runOptions.namespace, args[0], pgupgrade.PGUpgradeSettings{
				InitDBUser:             runOptions.postgresUser,
				CurrentPostgresVersion: runOptions.currentPostgresVersion,
				TargetPostgresVersion:  runOptions.targetPostgresVersion,
				InitDBArgs:             runOptions.extraInitDBArgs,
				DiskSize:               runOptions.newPVCDiskSize,
				TargetPVCName:          runOptions.targetPVCName,
			})
		},
	}

	AddPostgresUpgradeFlags(cmd.Flags(), runOptions)

	return cmd
}
