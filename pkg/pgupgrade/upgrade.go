package pgupgrade

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containerinfra/kube-pg-upgrade/pkg/kubeclient"
	"github.com/containerinfra/kube-pg-upgrade/pkg/kubescaler"
	"github.com/containerinfra/kube-pg-upgrade/pkg/kubevolumes"
	"github.com/containerinfra/kube-pg-upgrade/pkg/ptrs"
)

//go:embed scripts/prepare.sh
var upgradePrepareScript string

//go:embed scripts/posthook.sh
var postHookScript string

const (
	DefaultPostgresInitDBUser = "postgres"
)

type PGUpgradeSettings struct {
	InitDBArgs string
	DiskSize   string

	CurrentPostgresVersion string
	TargetPostgresVersion  string
	PostgresContainerName  string

	PVCName    string
	InitDBUser string

	TargetPVCName string
}

func (s *PGUpgradeSettings) GetInitDBUser() string {
	if s.InitDBUser == "" {
		return DefaultPostgresInitDBUser
	}
	return s.InitDBUser
}

func (s *PGUpgradeSettings) Validate() error {
	if s.TargetPostgresVersion == "" {
		return fmt.Errorf("missing target postgres version")
	}
	return nil
}

func RunPGUpgradeForDatabaseStatefulSet(ctx context.Context, targetNamespace, targetName string, settings PGUpgradeSettings) error {
	if err := settings.Validate(); err != nil {
		return err
	}

	kubeconfig, err := kubeclient.GetClientConfig()
	if err != nil {
		return err
	}
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		return err
	}
	k8sclient, err := kubeclient.GetClientWithConfig(config)
	if err != nil {
		return err
	}
	if targetNamespace == "" {
		contextNamespace, _, err := kubeconfig.Namespace()
		if err != nil {
			return err
		}
		targetNamespace = contextNamespace
	}

	var postgresContainer *v1.Container
	if settings.PostgresContainerName == "" {
		postgresContainer, err = autodiscoverPostgresContainer(ctx, k8sclient, targetNamespace, targetName)
		if err != nil {
			return fmt.Errorf("failed to auto discover postgres container: %w", err)
		}
	} else {
		postgresContainer, err = getContainerInStatefulset(ctx, k8sclient, targetNamespace, targetName, settings.PostgresContainerName)
		if err != nil {
			return fmt.Errorf("failed to find postgres container by name: %w", err)
		}
	}

	pgUser := strings.TrimSpace(getEnvValue(postgresContainer.Env, "POSTGRES_USER", "POSTGRES_INITSCRIPTS_USERNAME"))
	if pgUser == "" { // default fallback
		pgUser = settings.GetInitDBUser()
	}

	discoveredInitDBArguments := getEnvValue(postgresContainer.Env, "POSTGRES_INITDB_ARGS")
	databaseName := getEnvValue(postgresContainer.Env, "POSTGRES_DB")
	pgData := getEnvValue(postgresContainer.Env, "PGDATA")

	// TODO: figure out from statefulset
	// by default we have to use `data` from bitnami
	subpath := "data"

	extraInitDBArgs := strings.TrimSpace(fmt.Sprintf("%s %s", settings.InitDBArgs, discoveredInitDBArguments))
	fmt.Printf("---------\n")
	fmt.Printf("postgres user: %q\n", pgUser)
	fmt.Printf("postgres database: %q\n", databaseName)
	fmt.Printf("pgdata dir: %q\n", pgData)
	fmt.Printf("initdb-args: %q\n", extraInitDBArgs)
	fmt.Printf("---------\n")

	if len(postgresContainer.VolumeMounts) == 0 {
		return fmt.Errorf("missing volume mounts")
	}

	// TODO: not reliable, try and find the volume that uses a persistent disk
	mountName := postgresContainer.VolumeMounts[1].Name
	pvcName := fmt.Sprintf("%s-%s-0", mountName, targetName)
	targetPVCName := pvcName
	if settings.TargetPVCName != "" {
		targetPVCName = settings.TargetPVCName
	}

	if settings.CurrentPostgresVersion == "" {
		// attempt discovery from container image
		currentPostgresMajorVersion, err := AutoDiscoverPostgresVersionFromImage(postgresContainer.Image)
		if err != nil {
			return err
		}
		fmt.Printf("auto discovered current postgres version: %s\n", currentPostgresMajorVersion)
		settings.CurrentPostgresVersion = currentPostgresMajorVersion
	}

	pvc, err := kubevolumes.GetPersistentVolumeClaimAndCheckForVolumes(ctx, k8sclient, pvcName, targetNamespace)
	if err != nil {
		return err
	}
	if pvc == nil {
		return fmt.Errorf("pvc not found")
	}
	storageclass := getStorageClassForPVC(pvc)

	diskSize := getDiskSizeOrUsePVCDiskRequestSize(settings.DiskSize, pvc)
	if diskSize == "" {
		return fmt.Errorf("invalid disk size: must not be empty")
	}

	fmt.Printf("scaling down postgres statefulset...\n")

	scaler := kubescaler.NewKubeScalerWithClient(targetNamespace, k8sclient)
	err = scaler.ScaleStatefulSet(ctx, targetName, 0)
	if err != nil {
		return err
	}

	fmt.Printf("running pg_upgrade with init args: %q\n", fmt.Sprintf("-U %s %s", pgUser, extraInitDBArgs))

	err = RunPGDataMigration(ctx, k8sclient, targetNamespace, pvcName, targetPVCName, storageclass, diskSize, createUpgradeJobActionInput(settings, subpath, pgUser, extraInitDBArgs))
	if err != nil {
		return err
	}
	fmt.Printf("ran postgres upgrade succesfully\n")
	return nil
}

func createUpgradeJobActionInput(settings PGUpgradeSettings, subpath string, pgUser string, extraInitDBArgs string) JobActions {
	jobAction := JobActions{
		Name:           "pg-upgrade",
		Script:         upgradePrepareScript,
		PostHookScript: postHookScript,
		PrepareContainer: v1.Container{
			Name:  "prepare",
			Image: fmt.Sprintf("tianon/postgres-upgrade:%s-to-%s", settings.CurrentPostgresVersion, settings.TargetPostgresVersion),
			SecurityContext: &v1.SecurityContext{
				RunAsNonRoot: ptrs.False(),
			},
			Command: []string{"/bin/sh"},
			Args:    []string{fmt.Sprintf("/scripts/%s", PrepareScriptFileName)},
			VolumeMounts: []v1.VolumeMount{
				{
					Name: "old",

					MountPath: "/old",
					SubPath:   subpath,
				},
				{
					Name:      "new",
					MountPath: "/new",
					SubPath:   subpath,
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
			Image: fmt.Sprintf("tianon/postgres-upgrade:%s-to-%s", settings.CurrentPostgresVersion, settings.TargetPostgresVersion),
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
					SubPath:   subpath,
				},
				{
					Name:      "new",
					MountPath: fmt.Sprintf("/var/lib/postgresql/%s/data", settings.TargetPostgresVersion),
					SubPath:   subpath,
				},
			},
		},
		PostHookContainer: v1.Container{
			Name:  "posthook",
			Image: fmt.Sprintf("tianon/postgres-upgrade:%s-to-%s", settings.CurrentPostgresVersion, settings.TargetPostgresVersion),
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
					SubPath:   subpath,
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

func getContainerInStatefulset(ctx context.Context, k8sclient *kubernetes.Clientset, targetNamespace, targetName, containerName string) (*v1.Container, error) {
	sts, err := k8sclient.AppsV1().StatefulSets(targetNamespace).Get(ctx, targetName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, fmt.Errorf("could not find postgres container")
		}
		return nil, fmt.Errorf("could not find postgres container: %w", err)
	}
	if sts == nil {
		return nil, fmt.Errorf("could not find postgres statefulset")
	}

	containers := sts.Spec.Template.Spec.Containers

	if len(containers) == 0 {
		return nil, fmt.Errorf("no container found in statefulset")
	}

	var postgresContainer *v1.Container
	for _, container := range containers {
		if container.Name == containerName {
			c := container
			postgresContainer = &c
			fmt.Printf("found container: %q\n", container.Name)
		}
	}

	if postgresContainer == nil {
		return nil, fmt.Errorf("could not find postgres container")
	}
	return postgresContainer, nil
}

func autodiscoverPostgresContainer(ctx context.Context, k8sclient *kubernetes.Clientset, targetNamespace string, targetName string) (*v1.Container, error) {
	sts, err := k8sclient.AppsV1().StatefulSets(targetNamespace).Get(ctx, targetName, metav1.GetOptions{})
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, fmt.Errorf("could not find postgres container")
		}
		return nil, fmt.Errorf("could not find postgres container: %w", err)
	}
	if sts == nil {
		return nil, fmt.Errorf("no container found in statefulset")
	}

	containers := sts.Spec.Template.Spec.Containers

	if len(containers) == 0 {
		return nil, fmt.Errorf("could not find postgres container")
	}

	var postgresContainer *v1.Container
	for _, container := range containers {
		if isImage(container.Image, "/bitnami/postgresql:", "docker.io/bitnami/postgresql:", "/postgres:", "/postgresql:") {
			c := container
			postgresContainer = &c
			fmt.Printf("found container: %q\n", container.Name)
		}
	}

	if postgresContainer == nil {
		return nil, fmt.Errorf("could not find postgres container")
	}
	return postgresContainer, nil
}

func newPodEnvVar(name, value string) v1.EnvVar {
	return v1.EnvVar{
		Name:  name,
		Value: value,
	}
}

func getEnvValue(envs []v1.EnvVar, names ...string) string {
	for _, envItem := range envs {
		for _, name := range names {
			if envItem.Name == name {
				return envItem.Value
			}
		}
	}
	return ""
}

func getEnvValueFrom(envs []v1.EnvVar, name string) *v1.EnvVarSource {
	for _, envItem := range envs {
		if envItem.Name == name {
			return envItem.ValueFrom
		}
	}
	return nil
}

func isImage(containerImage string, images ...string) bool {
	for _, image := range images {
		if strings.Contains(containerImage, image) {
			return true
		}
	}
	return false

}
