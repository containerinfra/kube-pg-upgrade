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

type postgresPGUpgradeOptions struct {
	namespace string

	upgradeImage string

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

	sourcePVCName string
	targetPVCName string
	subPath       string
}

func newPostgresPGUpgradeOptions() *postgresPGUpgradeOptions {
	return &postgresPGUpgradeOptions{}
}

func AddPostgresStatefulSetUpgradeFlags(flagSet *flag.FlagSet, opts *postgresPGUpgradeOptions) {
	flagSet.StringVarP(&opts.namespace, "namespace", "n", "", "namespace of the postgres instance. Default is the configured namespace in your kubecontext.")
	flagSet.StringVar(&opts.upgradeImage, "upgrade-image", "tianon/postgres-upgrade", "Container image used to run pg_upgrade.")

	// PostgreSQL settings
	flagSet.StringVarP(&opts.postgresUser, "user", "u", "", "user used for initdb")
	flagSet.StringVarP(&opts.targetPostgresVersion, "version", "v", "", "target postgres major version. For example: 14, 15, 16, etc..")
	flagSet.StringVar(&opts.currentPostgresVersion, "current-version", "", "current version of the postgres database. Optional, will attempt auto discovery if left empty. For example: 9.6, 14, 15, 16, etc..")
	flagSet.StringVarP(&opts.extraInitDBArgs, "extra-initdb-args", "i", "", "provide any additional arguments for init-db. Use the same arguments that were provided when the database was originally created. See https://www.postgresql.org/docs/current/pgupgrade.html. Otherwise will attempt to auto detect.")

	// Disk settings
	flagSet.StringVar(&opts.newPVCDiskSize, "size", "", "New size. Example: 10G")
	flagSet.StringVar(&opts.subPath, "subpath", "", "subpath used for mounting the pvc")
	flagSet.StringVar(&opts.sourcePVCName, "source-pvc-name", "", "The name of the Persistent Volume Claim with the current postgres data. Optional, will attempt auto discovery if left empty.")
	flagSet.StringVar(&opts.targetPVCName, "target-pvc-name", "", "Target name of Persistent Volume Claim that will serve as the target for the upgraded postgres data. This is an optional setting, will use the source PVC name by default.")

	// Other
	flagSet.DurationVar(&opts.timeout, "timeout", 0*time.Second, "The length of time to wait before giving up, zero means infinite")
}

func AddPostgresPVCUpgradeFlags(flagSet *flag.FlagSet, opts *postgresPGUpgradeOptions) {
	flagSet.StringVarP(&opts.namespace, "namespace", "n", "", "namespace of the postgres instance. Default is the configured namespace in your kubecontext.")
	flagSet.StringVar(&opts.upgradeImage, "upgrade-image", "tianon/postgres-upgrade", "Container image used to run pg_upgrade.")

	// PostgreSQL settings
	flagSet.StringVarP(&opts.postgresUser, "user", "u", "", "user used for initdb")
	flagSet.StringVarP(&opts.targetPostgresVersion, "version", "v", "", "target postgres major version. For example: 14, 15, 16, etc..")
	flagSet.StringVar(&opts.currentPostgresVersion, "current-version", "", "current version of the postgres database. For example: 9.6, 14, 15, 16, etc..")
	flagSet.StringVarP(&opts.extraInitDBArgs, "extra-initdb-args", "i", "", "provide any additional arguments for init-db. Use the same arguments that were provided when the database was originally created. See https://www.postgresql.org/docs/current/pgupgrade.html.")

	// Disk settings
	flagSet.StringVar(&opts.newPVCDiskSize, "size", "", "New size. Example: 10G")
	flagSet.StringVar(&opts.subPath, "subpath", "", "subpath used for mounting the pvc")
	flagSet.StringVar(&opts.targetPVCName, "target-pvc-name", "", "Target name of Persistent Volume Claim that will serve as the target for the upgraded postgres data. This is an optional setting, will use the source PVC name by default.")

	// Other
	flagSet.DurationVar(&opts.timeout, "timeout", 0*time.Second, "The length of time to wait before giving up, zero means infinite")
}

//go:embed examples/upgrade.txt
var pgUpgradeStatefulSetExamples string

// NewUpgradePostgresStatefulSetCmd
func NewUpgradePostgresStatefulSetCmd(runOptions *postgresPGUpgradeOptions) *cobra.Command {
	if runOptions == nil {
		runOptions = newPostgresPGUpgradeOptions()
	}

	var cmd = &cobra.Command{
		Use:     "statefulset <statefulset>",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"sts"},
		Short:   ``,
		Long:    ``,
		Example: pgUpgradeStatefulSetExamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			if runOptions.timeout > 0 {
				timeoutctx, cancelTimeout := context.WithTimeoutCause(ctx, runOptions.timeout, fmt.Errorf("upgrade did not complete within configured timeout (%s)", runOptions.timeout.String()))
				defer cancelTimeout()
				ctx = timeoutctx
			}

			upgrader, err := pgupgrade.NewPGUpgradeRunner(runOptions.namespace, pgupgrade.PGUpgradeSettings{
				UpgradeImage: runOptions.upgradeImage,

				InitDBUser:             runOptions.postgresUser,
				CurrentPostgresVersion: runOptions.currentPostgresVersion,
				TargetPostgresVersion:  runOptions.targetPostgresVersion,
				InitDBArgs:             runOptions.extraInitDBArgs,

				DiskSize:      runOptions.newPVCDiskSize,
				TargetPVCName: runOptions.targetPVCName,
				SourcePVCName: runOptions.sourcePVCName,
				SubPath:       runOptions.subPath,
			})
			if err != nil {
				return err
			}
			return upgrader.RunPGUpgradeForDatabaseStatefulSet(ctx, args[0])
		},
	}

	AddPostgresStatefulSetUpgradeFlags(cmd.Flags(), runOptions)

	return cmd
}

//go:embed examples/upgrade-pvc.txt
var pgUpgradePVCxamples string

// NewUpgradePostgresPVCCmd
func NewUpgradePostgresPVCCmd(runOptions *postgresPGUpgradeOptions) *cobra.Command {
	if runOptions == nil {
		runOptions = newPostgresPGUpgradeOptions()
	}

	var cmd = &cobra.Command{
		Use:     "pvc <name>",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"persistent-volume-claim"},
		Short:   ``,
		Long:    ``,
		Example: pgUpgradePVCxamples,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			if runOptions.timeout > 0 {
				timeoutctx, cancelTimeout := context.WithTimeoutCause(ctx, runOptions.timeout, fmt.Errorf("upgrade did not complete within configured timeout (%s)", runOptions.timeout.String()))
				defer cancelTimeout()
				ctx = timeoutctx
			}

			upgrader, err := pgupgrade.NewPGUpgradeRunner(runOptions.namespace, pgupgrade.PGUpgradeSettings{
				UpgradeImage: runOptions.upgradeImage,

				InitDBUser:             runOptions.postgresUser,
				CurrentPostgresVersion: runOptions.currentPostgresVersion,
				TargetPostgresVersion:  runOptions.targetPostgresVersion,
				InitDBArgs:             runOptions.extraInitDBArgs,

				DiskSize:      runOptions.newPVCDiskSize,
				TargetPVCName: runOptions.targetPVCName,
				SourcePVCName: args[0],
				SubPath:       runOptions.subPath,
			})
			if err != nil {
				return err
			}
			return upgrader.RunPGUpgradeForDatabasePVC(ctx)
		},
	}

	AddPostgresPVCUpgradeFlags(cmd.Flags(), runOptions)

	return cmd
}
