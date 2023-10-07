package kubesecrethelper

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CreateSecretOptions struct {
	Name        string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
	StringData  map[string]string
	Data        map[string][]byte
}

func CreateSecret(opts CreateSecretOptions) *v1.Secret {
	return &v1.Secret{
		Type: v1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			// The component and tier labels are useful for quickly identifying the control plane Pods when doing a .List()
			// against Pods in the kube-system namespace. Can for example be used together with the WaitForPodsWithLabel function
			Labels:      opts.Labels,
			Annotations: opts.Annotations,
		},
		StringData: opts.StringData,
		Data:       opts.Data,
	}
}

func ExistSecret(ctx context.Context, client kubernetes.Interface, namespace, name string) (bool, error) {
	if _, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func CreateOrUpdateSecret(ctx context.Context, client kubernetes.Interface, secret *v1.Secret) error {
	if _, err := client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "unable to create secret")
		}

		if _, err := client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "unable to update secret")
		}
	}
	return nil
}

func CreateOrUpdateConfigMap(ctx context.Context, client kubernetes.Interface, configmap *v1.ConfigMap) error {
	if _, err := client.CoreV1().ConfigMaps(configmap.ObjectMeta.Namespace).Create(ctx, configmap, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "unable to create configmap")
		}

		if _, err := client.CoreV1().ConfigMaps(configmap.ObjectMeta.Namespace).Update(ctx, configmap, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "unable to update configmap")
		}
	}
	return nil
}
