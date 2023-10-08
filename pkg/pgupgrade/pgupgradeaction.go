package pgupgrade

import (
	"fmt"

	"github.com/containerinfra/kube-pg-upgrade/pkg/ptrs"
	v1 "k8s.io/api/core/v1"
)

func createUpgradeJobActionInput(settings PGUpgradeSettings, sourceSubPath, targetSubPath string, pgUser string, extraInitDBArgs string) JobActions {
	jobAction := JobActions{
		Name:           "pg-upgrade",
		Script:         upgradePrepareScript,
		PostHookScript: postHookScript,
		PrepareContainer: v1.Container{
			Name:  "prepare",
			Image: settings.GetUpgradeImage(),
			SecurityContext: &v1.SecurityContext{
				RunAsNonRoot: ptrs.False(),
			},
			Command: []string{"/bin/sh"},
			Args:    []string{fmt.Sprintf("/scripts/%s", PrepareScriptFileName)},
			VolumeMounts: []v1.VolumeMount{
				{
					Name: "old",

					MountPath: "/old",
					SubPath:   sourceSubPath,
				},
				{
					Name:      "new",
					MountPath: "/new",
					SubPath:   targetSubPath,
				},
				{
					Name:      "scripts",
					MountPath: "/scripts/",
					ReadOnly:  true,
				},
			},
		},
		JobContainer: v1.Container{
			Name:  "upgrade-postgres",
			Image: settings.GetUpgradeImage(),
			SecurityContext: &v1.SecurityContext{
				RunAsNonRoot: ptrs.False(),
			},
			Env: []v1.EnvVar{
				newPodEnvVar("PGUSER", pgUser),
				newPodEnvVar("POSTGRES_USER", pgUser),
				newPodEnvVar("POSTGRES_INITDB_ARGS", fmt.Sprintf("-U %s %s", pgUser, extraInitDBArgs)),
			},
			VolumeMounts: []v1.VolumeMount{
				{
					Name: "old",

					MountPath: fmt.Sprintf("/var/lib/postgresql/%s/data", settings.CurrentPostgresVersion),
					SubPath:   sourceSubPath,
				},
				{
					Name:      "new",
					MountPath: fmt.Sprintf("/var/lib/postgresql/%s/data", settings.TargetPostgresVersion),
					SubPath:   targetSubPath,
				},
			},
		},
		PostHookContainer: v1.Container{
			Name:  "posthook",
			Image: settings.GetUpgradeImage(),
			SecurityContext: &v1.SecurityContext{
				RunAsUser:    ptrs.Int64(0),
				RunAsGroup:   ptrs.Int64(0),
				RunAsNonRoot: ptrs.False(),
			},
			Command: []string{"/bin/sh"},
			Args:    []string{fmt.Sprintf("/scripts/%s", PostHookScriptFileName)},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "new",
					MountPath: "/new",
					SubPath:   targetSubPath,
				},
				{
					Name:      "scripts",
					MountPath: "/scripts/",
					ReadOnly:  true,
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
