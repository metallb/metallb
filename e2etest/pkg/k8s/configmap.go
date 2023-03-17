// SPDX-License-Identifier:Apache-2.0

package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

func CreateConfigmap(cs clientset.Interface, name, namespace string, data map[string]string) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	_, err := cs.CoreV1().ConfigMaps(namespace).Create(context.Background(), &cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func RemoveConfigmap(cs clientset.Interface, name, namespace string) error {
	err := cs.CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
