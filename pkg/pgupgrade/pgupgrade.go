package pgupgrade

import (
	"context"
	_ "embed"
	"fmt"

	v1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/util/retry"

	"github.com/containerinfra/kube-pg-upgrade/pkg/kubesecrethelper"
	"github.com/containerinfra/kube-pg-upgrade/pkg/kubevolumes"
	"github.com/containerinfra/kube-pg-upgrade/pkg/podrunner"
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
	UpgradeImage string

	InitDBArgs string
	DiskSize   string

	CurrentPostgresVersion string
	TargetPostgresVersion  string
	PostgresContainerName  string

	PVCName    string
	InitDBUser string

	SourcePVCName string
	TargetPVCName string
	SubPath       string
}

func (s *PGUpgradeSettings) GetUpgradeImage() string {
	return fmt.Sprintf("%s:%s-to-%s", s.UpgradeImage, s.CurrentPostgresVersion, s.TargetPostgresVersion)
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

const (
	PostHookScriptFileName = "posthook.sh"
	PrepareScriptFileName  = "prepare.sh"
)

type JobActions struct {
	Name              string
	Script            string
	PostHookScript    string
	JobContainer      v1.Container
	PrepareContainer  v1.Container
	PostHookContainer v1.Container
}

func RunPGDataMigration(ctx context.Context, k8sClient *kubernetes.Clientset, namespace, sourcePersistenVolumeName, targetPVCName, storageClassName string, newSize string, jobaction JobActions) error {
	upgradePodName := Truncate(jobaction.Name+sourcePersistenVolumeName, 63)
	upgradeTargetPersistentVolumeTempName := Truncate("tmp-"+sourcePersistenVolumeName, 63)

	if err := kubevolumes.ValidateStorageClassExists(ctx, k8sClient, storageClassName); err != nil {
		return err
	}

	pvc, err := kubevolumes.GetPersistentVolumeClaimAndWaitForVolume(ctx, k8sClient, namespace, sourcePersistenVolumeName)
	if err != nil {
		return err
	}

	storageSize, err := resource.ParseQuantity(newSize)
	if err != nil {
		return fmt.Errorf("cannot parse size into quantity: %v", err)
	}

	scriptSecretName := upgradePodName

	err = kubesecrethelper.CreateOrUpdateSecret(ctx, k8sClient, kubesecrethelper.CreateSecret(kubesecrethelper.CreateSecretOptions{
		Name:      scriptSecretName,
		Namespace: namespace,
		Data: map[string][]byte{
			PrepareScriptFileName:  []byte(jobaction.Script),
			PostHookScriptFileName: []byte(jobaction.PostHookScript),
		},
	}))
	if err != nil {
		return err
	}
	// make sure we remove the secret once we are done with it
	defer k8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), scriptSecretName, metav1.DeleteOptions{})

	err = kubevolumes.CreatePersistentVolumeClaim(ctx, k8sClient, upgradeTargetPersistentVolumeTempName, namespace, storageClassName, storageSize)
	if err != nil {
		if !kubeerrors.IsAlreadyExists(err) {
			return err
		}
		fmt.Printf("Using existing pvc %q\n", upgradeTargetPersistentVolumeTempName)
	} else {
		fmt.Printf("Temporary pvc %q created\n", upgradeTargetPersistentVolumeTempName)
	}

	mismatch := v1.FSGroupChangeOnRootMismatch
	r := podrunner.NewPodRunner(k8sClient)

	// run the pg-upgrade job
	err = r.RunPod(ctx, namespace, upgradePodName, v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      upgradePodName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsNonRoot:        ptrs.False(),
				FSGroupChangePolicy: &mismatch,
			},
			InitContainers: []v1.Container{
				jobaction.PrepareContainer,
			},
			Containers: []v1.Container{
				jobaction.JobContainer,
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				kubevolumes.NewPersistentVolumeClaimVolume("old", sourcePersistenVolumeName, false),
				kubevolumes.NewPersistentVolumeClaimVolume("new", upgradeTargetPersistentVolumeTempName, false),
				kubevolumes.NewVolumeFromSecret("scripts", scriptSecretName),
			},
		},
	})
	if err != nil {
		return err
	}

	// SWITCHING DISKS AROUND

	tmpPVC, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, upgradeTargetPersistentVolumeTempName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get persistent volume claim%q: %w", upgradeTargetPersistentVolumeTempName, err)
	}

	err = retry.OnError(retry.DefaultBackoff, RetryAllErrorsFn(ctx), func() error {
		err = kubevolumes.SetPVReclaimPolicyToRetain(ctx, k8sClient, pvc)
		if err != nil {
			return err
		}
		return kubevolumes.SetPVReclaimPolicyToRetain(ctx, k8sClient, tmpPVC)
	})

	err = retry.OnError(retry.DefaultBackoff, RetryAllErrorsFn(ctx), func() error {
		err = kubevolumes.SetPVReclaimPolicyToRetain(ctx, k8sClient, pvc)
		if err != nil {
			return err
		}
		return kubevolumes.SetPVReclaimPolicyToRetain(ctx, k8sClient, tmpPVC)
	})
	if err != nil {
		return err
	}

	// make sure the persistent volumes are set correctly
	err = cleanupPersistentVolumes(ctx, k8sClient, namespace, upgradeTargetPersistentVolumeTempName, sourcePersistenVolumeName)
	if err != nil {
		return err
	}

	// Create the new target PVC using the targetPVCName
	err = createFinalTargetPVCWithPV(ctx, k8sClient, tmpPVC, targetPVCName, namespace, storageClassName, storageSize)
	if err != nil {
		return err
	}
	err = retry.OnError(retry.DefaultBackoff, RetryAllErrorsFn(ctx), func() error {
		return validatePVCCreationCompleted(ctx, k8sClient, targetPVCName, namespace, tmpPVC, storageClassName, sourcePersistenVolumeName, pvc)
	})
	if err != nil {
		return err
	}

	postHookPodName := Truncate(fmt.Sprintf("post-upgrade-%s-%s", jobaction.Name, sourcePersistenVolumeName), 63)

	fmt.Printf("[pg_upgrade] running the post upgrade hook container %q...\n", postHookPodName)
	err = r.RunPod(ctx, namespace, postHookPodName, v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      postHookPodName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsNonRoot:        ptrs.False(),
				FSGroupChangePolicy: &mismatch,
			},
			Containers: []v1.Container{
				jobaction.PostHookContainer,
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				kubevolumes.NewPersistentVolumeClaimVolume("new", targetPVCName, false),
				kubevolumes.NewVolumeFromSecret("scripts", scriptSecretName),
			},
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("[pg_upgrade] completed running the post upgrade hook container\n")
	return nil
}

func validatePVCCreationCompleted(ctx context.Context, k8sClient *kubernetes.Clientset, targetPVCName string, namespace string, tmpPVC *v1.PersistentVolumeClaim, storageClassName string, sourcePersistenVolumeName string, pvc *v1.PersistentVolumeClaim) error {
	finalPVC, err := kubevolumes.GetPersistentVolumeClaimAndWaitForVolume(ctx, k8sClient, namespace, targetPVCName)
	if err != nil {
		return fmt.Errorf("failed to get new persistent volume claim%q: %w", targetPVCName, err)
	}

	if finalPVC.Status.Phase != v1.ClaimBound {
		return fmt.Errorf("new persistent volume claim is not bound! %q", finalPVC.Name)
	}

	if finalPVC.Spec.VolumeName != tmpPVC.Spec.VolumeName {
		return fmt.Errorf("new persistent volume claim %q is not bound to the new persistentvolume! %q", finalPVC.Name, tmpPVC.Spec.VolumeName)
	}

	if *finalPVC.Spec.StorageClassName != storageClassName {
		return fmt.Errorf("new persistent volume claim %q has the storageclass %q and not the given storageclass %q", sourcePersistenVolumeName, *finalPVC.Spec.StorageClassName, storageClassName)
	}
	fmt.Printf("Data in %q succesfully migrated to %q bound to PVC %q with storageclass %q\n", pvc.Spec.VolumeName, finalPVC.Spec.VolumeName, finalPVC.Name, *finalPVC.Spec.StorageClassName)
	return nil
}

func createFinalTargetPVCWithPV(ctx context.Context, k8sClient *kubernetes.Clientset, tmpPVC *v1.PersistentVolumeClaim, targetPVCName string, namespace string, storageClassName string, storageSize resource.Quantity) error {
	err := retry.OnError(retry.DefaultBackoff, RetryAllErrorsFn(ctx), func() error {
		err := kubevolumes.RemoveClaimRefOfPV(ctx, k8sClient, tmpPVC)
		if err != nil {
			return err
		}
		claimRef := v1.ObjectReference{Name: targetPVCName, Namespace: namespace}
		return kubevolumes.SetClaimRefOfPV(ctx, k8sClient, tmpPVC.Spec.VolumeName, claimRef)
	})
	if err != nil {
		return err
	}

	err = retry.OnError(retry.DefaultBackoff, RetryAllErrorsFn(ctx), func() error {
		return kubevolumes.CreatePersistentVolumeClaim(ctx, k8sClient, targetPVCName, namespace, storageClassName, storageSize)
	})
	if err != nil {
		return err
	}
	fmt.Printf("Created final pvc %q\n", targetPVCName)
	return nil
}

func cleanupPersistentVolumes(ctx context.Context, k8sClient *kubernetes.Clientset, namespace string, tmpPVCName string, pvcName string) error {
	err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, tmpPVCName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete persistent volume claim%q: %w", tmpPVCName, err)
	}
	fmt.Printf("Deleting temp pvc %q (persistent volume is marked as retain)\n", tmpPVCName)

	err = k8sClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete persistent volume claim%q: %w", pvcName, err)
	}
	fmt.Printf("Deleting source pvc: %s (persistent volume is marked as retain)\n", pvcName)

	err = kubevolumes.WaitForPVCToBeDeleted(ctx, k8sClient, namespace, pvcName)
	if err != nil {
		return err
	}
	return nil
}

func RetryAllErrorsFn(ctx context.Context) func(err error) bool {
	return func(err error) bool {
		return ctx.Err() == nil
	}
}

// Truncate returns the first n runes of s.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}
