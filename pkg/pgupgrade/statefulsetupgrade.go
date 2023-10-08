package pgupgrade

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/containerinfra/kube-pg-upgrade/pkg/kubeclient"
	"github.com/containerinfra/kube-pg-upgrade/pkg/kubescaler"
	"github.com/containerinfra/kube-pg-upgrade/pkg/kubevolumes"
)

type PGUpgradeRunner struct {
	namespace string
	k8sclient *kubernetes.Clientset
	settings  PGUpgradeSettings
}

func NewPGUpgradeRunner(namespace string, settings PGUpgradeSettings) (*PGUpgradeRunner, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	kubeconfig, err := kubeclient.GetClientConfig()
	if err != nil {
		return nil, err
	}
	clientconfig, err := kubeconfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	k8sclient, err := kubeclient.GetClientWithConfig(clientconfig)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		contextNamespace, _, err := kubeconfig.Namespace()
		if err != nil {
			return nil, err
		}
		namespace = contextNamespace
	}

	return &PGUpgradeRunner{
		namespace: namespace,
		k8sclient: k8sclient,
		settings:  settings,
	}, nil
}

func (r *PGUpgradeRunner) RunPGUpgradeForDatabaseStatefulSet(ctx context.Context, targetStatefulSetName string) error {
	var err error
	var postgresContainer *v1.Container

	if r.settings.PostgresContainerName == "" {
		postgresContainer, err = autodiscoverPostgresContainer(ctx, r.k8sclient, r.namespace, targetStatefulSetName)
		if err != nil {
			return fmt.Errorf("failed to auto discover postgres container: %w", err)
		}
	} else {
		postgresContainer, err = getContainerInStatefulset(ctx, r.k8sclient, r.namespace, targetStatefulSetName, r.settings.PostgresContainerName)
		if err != nil {
			return fmt.Errorf("failed to find postgres container by name: %w", err)
		}
	}

	pgUser := strings.TrimSpace(getEnvValue(postgresContainer.Env, "POSTGRES_USER", "POSTGRES_INITSCRIPTS_USERNAME"))
	if pgUser == "" { // default fallback
		pgUser = r.settings.GetInitDBUser()
	}

	discoveredInitDBArguments := getEnvValue(postgresContainer.Env, "POSTGRES_INITDB_ARGS")

	extraInitDBArgs := strings.TrimSpace(fmt.Sprintf("%s %s", r.settings.InitDBArgs, discoveredInitDBArguments))
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

	if len(postgresContainer.VolumeMounts) == 0 {
		return fmt.Errorf("missing volume mounts")
	}

	// TODO: not reliable, try and find the volume that uses a persistent disk
	mountName := postgresContainer.VolumeMounts[1].Name
	sourcePVCName := fmt.Sprintf("%s-%s-0", mountName, targetStatefulSetName)

	if r.settings.SourcePVCName != "" {
		sourcePVCName = r.settings.SourcePVCName
	}

	targetPVCName := sourcePVCName
	if r.settings.TargetPVCName != "" {
		targetPVCName = r.settings.TargetPVCName
	}

	if r.settings.CurrentPostgresVersion == "" {
		// attempt discovery from container image
		currentPostgresMajorVersion, err := AutoDiscoverPostgresVersionFromImage(postgresContainer.Image)
		if err != nil {
			return err
		}
		fmt.Printf("auto discovered current postgres version: %s\n", currentPostgresMajorVersion)
		r.settings.CurrentPostgresVersion = currentPostgresMajorVersion
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

	fmt.Printf("scaling down postgres statefulset...\n")
	scaler := kubescaler.NewKubeScalerWithClient(r.namespace, r.k8sclient)
	err = scaler.ScaleStatefulSet(ctx, targetStatefulSetName, 0)
	if err != nil {
		return err
	}

	fmt.Printf("running pg_upgrade with init args: %q\n", fmt.Sprintf("-U %s %s", pgUser, extraInitDBArgs))

	err = RunPGDataMigration(ctx, r.k8sclient, r.namespace, sourcePVCName, targetPVCName, storageclass, diskSize, createUpgradeJobActionInput(r.settings, subpath, subpath, pgUser, extraInitDBArgs))
	if err != nil {
		return err
	}
	fmt.Printf("ran postgres upgrade succesfully\n")
	return nil
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

func isImage(containerImage string, images ...string) bool {
	for _, image := range images {
		if strings.Contains(containerImage, image) {
			return true
		}
	}
	return false
}
