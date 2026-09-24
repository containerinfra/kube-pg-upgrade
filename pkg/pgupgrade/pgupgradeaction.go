package pgupgrade

import (
	"fmt"
	"path"

	v1 "k8s.io/api/core/v1"

	"github.com/containerinfra/kube-pg-upgrade/pkg/ptrs"
)

func createUpgradeJobActionInput(settings PGUpgradeSettings, sourceSubPath, targetSubPath string, pgUser string, extraInitDBArgs string) JobActions {
	// Mount PVC roots (no VolumeMount.SubPath). Cluster files live in a *subdirectory*
	// named by the subpath setting (default "data"). That way initdb/pg_ctl can chmod
	// PGDATA — chmod on a Kubernetes subPath mount point fails with EPERM.
	if sourceSubPath == "" {
		sourceSubPath = "data"
	}
	if targetSubPath == "" {
		targetSubPath = "data"
	}

	oldRoot := fmt.Sprintf("/var/lib/postgresql/%s", settings.CurrentPostgresVersion)
	newRoot := fmt.Sprintf("/var/lib/postgresql/%s", settings.TargetPostgresVersion)
	oldDataDir := path.Join(oldRoot, sourceSubPath)
	newDataDir := path.Join(newRoot, targetSubPath)

	jobAction := JobActions{
		Name:            "pg-upgrade",
		Script:          upgradePrepareScript,
		PostHookScript:  postHookScript,
		SecurityContext: settings.SecurityContext,
		PrepareContainer: v1.Container{
			Name:  "prepare",
			Image: settings.GetUpgradeImage(),
			SecurityContext: &v1.SecurityContext{
				RunAsNonRoot:           &settings.SecurityContext.RunAsNonRoot,
				RunAsUser:              &settings.SecurityContext.RunAsUser,
				RunAsGroup:             &settings.SecurityContext.RunAsGroup,
				ReadOnlyRootFilesystem: ptrs.True(),
			},
			Command: []string{"/bin/sh"},
			Args:    []string{fmt.Sprintf("/scripts/%s", PrepareScriptFileName)},
			Env: []v1.EnvVar{
				newPodEnvVar("OLD_DATA", "/old/"+sourceSubPath),
				newPodEnvVar("NEW_DATA", "/new/"+targetSubPath),
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "old",
					MountPath: "/old",
				},
				{
					Name:      "new",
					MountPath: "/new",
				},
				{
					Name:      "scripts",
					MountPath: "/scripts/",
					ReadOnly:  true,
				},
				{
					Name:      "tmp",
					MountPath: "/tmp",
				},
				{
					Name:      "postgresql-run",
					MountPath: "/var/run/postgresql",
				},
			},
		},
		JobContainer: v1.Container{
			Name:  "upgrade-postgres",
			Image: settings.GetUpgradeImage(),
			SecurityContext: &v1.SecurityContext{
				RunAsNonRoot:           &settings.SecurityContext.RunAsNonRoot,
				RunAsUser:              &settings.SecurityContext.RunAsUser,
				RunAsGroup:             &settings.SecurityContext.RunAsGroup,
				ReadOnlyRootFilesystem: ptrs.True(),
			},
			Env: []v1.EnvVar{
				newPodEnvVar("PGUSER", pgUser),
				newPodEnvVar("POSTGRES_USER", pgUser),
				newPodEnvVar("POSTGRES_INITDB_ARGS", fmt.Sprintf("-U %s %s", pgUser, extraInitDBArgs)),
				newPodEnvVar("PGDATAOLD", oldDataDir),
				newPodEnvVar("PGDATANEW", newDataDir),
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "old",
					MountPath: oldRoot,
				},
				{
					Name:      "new",
					MountPath: newRoot,
				},
				{
					Name:      "tmp",
					MountPath: "/tmp",
				},
				{
					Name:      "postgresql-run",
					MountPath: "/var/run/postgresql",
				},
			},
		},
		PostHookContainer: v1.Container{
			Name:  "posthook",
			Image: settings.GetUpgradeImage(),
			SecurityContext: &v1.SecurityContext{
				RunAsNonRoot:           &settings.SecurityContext.RunAsNonRoot,
				RunAsUser:              &settings.SecurityContext.RunAsUser,
				RunAsGroup:             &settings.SecurityContext.RunAsGroup,
				ReadOnlyRootFilesystem: ptrs.True(),
			},
			Command: []string{"/bin/sh"},
			Args:    []string{fmt.Sprintf("/scripts/%s", PostHookScriptFileName)},
			Env: []v1.EnvVar{
				newPodEnvVar("NEW_DATA", "/new/"+targetSubPath),
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "new",
					MountPath: "/new",
				},
				{
					Name:      "scripts",
					MountPath: "/scripts/",
					ReadOnly:  true,
				},
				{
					Name:      "tmp",
					MountPath: "/tmp",
				},
				{
					Name:      "postgresql-run",
					MountPath: "/var/run/postgresql",
				},
			},
		},
	}
	return jobAction
}

func getDiskSizeOrUsePVCDiskRequestSize(diskSize string, pvc *v1.PersistentVolumeClaim) string {
	if pvc != nil && pvc.Spec.Resources.Requests.Storage() != nil {
		diskSize = pvc.Spec.Resources.Requests.Storage().String()
	}
	return diskSize
}

func getStorageClassForPVC(pvc *v1.PersistentVolumeClaim) string {
	storageclass := ""
	if pvc.Spec.StorageClassName != nil {
		storageclass = *pvc.Spec.StorageClassName
	}
	return storageclass
}
