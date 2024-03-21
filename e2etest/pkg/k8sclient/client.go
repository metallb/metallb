// SPDX-License-Identifier:Apache-2.0

package k8sclient

import (
	clientset "k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"

	restclient "k8s.io/client-go/rest"
)

func New() *clientset.Clientset {
	config := ctrl.GetConfigOrDie()
	res, err := clientset.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return res
}

func RestConfig() *restclient.Config {
	return ctrl.GetConfigOrDie()
}
