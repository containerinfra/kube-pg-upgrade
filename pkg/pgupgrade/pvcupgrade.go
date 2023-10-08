package pgupgrade

import (
	"context"
	"fmt"

	"github.com/containerinfra/kube-pg-upgrade/pkg/kubevolumes"
)

func (r *PGUpgradeRunner) RunPGUpgradeForDatabasePVC(ctx context.Context) error {
	pgUser := r.settings.GetInitDBUser()
	extraInitDBArgs := r.settings.InitDBArgs

	fmt.Printf("---------\n")
	fmt.Printf("postgres user: %q\n", pgUser)
	fmt.Printf("initdb-args: %q\n", extraInitDBArgs)
	fmt.Printf("---------\n")

	// TODO: figure out from statefulset
	// by default we have to use `data` from bitnami
	subpath := "data"
	if r.settings.SubPath != "" {
		subpath = r.settings.SubPath
	}

	sourcePVCName := r.settings.SourcePVCName
	if sourcePVCName == "" {
		return fmt.Errorf("source pvc name must not be empty")
	}

	targetPVCName := r.settings.TargetPVCName
	if targetPVCName == "" {
		return fmt.Errorf("target pvc name must not be empty")
	}

	if r.settings.CurrentPostgresVersion == "" {
		return fmt.Errorf("must provide current postgres version")
	}

	// if versions are equal, we don't have to do anything
	if r.settings.CurrentPostgresVersion == r.settings.TargetPostgresVersion {
		return fmt.Errorf("current postgres version is equal to target postgres version: %q", r.settings.CurrentPostgresVersion)
	}

	sourcePVC, err := kubevolumes.GetPersistentVolumeClaimAndWaitForVolume(ctx, r.k8sclient, r.namespace, sourcePVCName)
	if err != nil {
		return err
	}
	if sourcePVC == nil {
		return fmt.Errorf("source pvc not found")
	}
	storageclass := getStorageClassForPVC(sourcePVC)

	diskSize := getDiskSizeOrUsePVCDiskRequestSize(r.settings.DiskSize, sourcePVC)
	if diskSize == "" {
		return fmt.Errorf("invalid disk size: must not be empty")
	}

	fmt.Printf("running pg_upgrade with init args: %q\n", fmt.Sprintf("-U %s %s", pgUser, extraInitDBArgs))
	err = RunPGDataMigration(ctx, r.k8sclient, r.namespace, sourcePVCName, targetPVCName, storageclass, diskSize, createUpgradeJobActionInput(r.settings, subpath, subpath, pgUser, extraInitDBArgs))
	if err != nil {
		return err
	}
	fmt.Printf("ran postgres upgrade succesfully\n")
	return nil
}
