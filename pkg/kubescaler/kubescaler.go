package kubescaler

import (
	"context"

	"github.com/containerinfra/kube-pg-upgrade/pkg/kubeclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type KubeScaler struct {
	client    kubernetes.Interface
	namespace string
}

func NewKubeScaler(namespace string) (*KubeScaler, error) {
	k8sclient, err := kubeclient.GetClient()
	if err != nil {
		return nil, err
	}

	return &KubeScaler{
		client:    k8sclient,
		namespace: namespace,
	}, nil
}

func NewKubeScalerOrDie(namespace string) *KubeScaler {
	client, err := NewKubeScaler(namespace)
	if err != nil {
		panic(err)
	}
	return client
}

func NewKubeScalerWithClient(namespace string, client kubernetes.Interface) *KubeScaler {
	return &KubeScaler{
		client:    client,
		namespace: namespace,
	}
}

func (a *KubeScaler) ScaleDeployment(ctx context.Context, deploymentName string, replicas int32) error {
	scale, err := a.client.AppsV1().Deployments(a.namespace).GetScale(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	scale.Spec.Replicas = replicas
	_, err = a.client.AppsV1().Deployments(a.namespace).UpdateScale(ctx, deploymentName, scale, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (a *KubeScaler) ScaleStatefulSet(ctx context.Context, statefulSetName string, replicas int32) error {
	scale, err := a.client.AppsV1().StatefulSets(a.namespace).GetScale(ctx, statefulSetName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	scale.Spec.Replicas = replicas
	_, err = a.client.AppsV1().StatefulSets(a.namespace).UpdateScale(ctx, statefulSetName, scale, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
