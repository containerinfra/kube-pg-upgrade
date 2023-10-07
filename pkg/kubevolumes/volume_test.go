package kubevolumes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/containerinfra/kube-pg-upgrade/pkg/ptrs"
)

func TestCreatePersistentVolumeClaim(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	assert.NotNil(t, k8sClient)

	pvcName := "test-pvc"
	namespace := "default"
	storageClass := "ebs"

	err := CreatePersistentVolumeClaim(context.TODO(), k8sClient, pvcName, namespace, storageClass, resource.MustParse("1"))
	if !assert.NoError(t, err, "should not receive an error when creating the pvc") {
		t.FailNow()
	}

	testPVC, err := k8sClient.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
	if !assert.NoError(t, err, "should not recieve an error when creating the pvc") {
		t.FailNow()
	}
	assert.Equal(t, pvcName, testPVC.Name)
	assert.Equal(t, namespace, testPVC.Namespace)
	assert.Equal(t, ptrs.String(storageClass), testPVC.Spec.StorageClassName)
	assert.Equal(t, []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}, testPVC.Spec.AccessModes)
	assert.Equal(t, resource.MustParse("1"), *testPVC.Spec.Resources.Requests.Storage())
}

func TestNewVolumeMount(t *testing.T) {

	volumeMount := v1.VolumeMount{
		Name:      "name",
		MountPath: "path",
		ReadOnly:  false,
	}

	assert.Equal(t, volumeMount, NewVolumeMount("name", "path", false))
	assert.NotEqual(t, volumeMount, NewVolumeMount("name", "path", true))
	assert.NotEqual(t, volumeMount, NewVolumeMount("name", "nopath", false))
	assert.NotEqual(t, volumeMount, NewVolumeMount("noname", "path", false))
}

func TestNewPersistentVolumeClaimVolume(t *testing.T) {
	volume := v1.Volume{
		Name: "name",
		VolumeSource: v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
				ClaimName: "claimName",
				ReadOnly:  false,
			},
		},
	}

	assert.Equal(t, volume, NewPersistentVolumeClaimVolume("name", "claimName", false))
	assert.NotEqual(t, volume, NewPersistentVolumeClaimVolume("name", "claimName", true))
	assert.NotEqual(t, volume, NewPersistentVolumeClaimVolume("name", "noclaimName", false))
	assert.NotEqual(t, volume, NewPersistentVolumeClaimVolume("noname", "claimName", false))
}
