// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package kubernetes builds REST clients and optional snapshot clients for apiserver access.
package kubernetes

import (
	"fmt"

	snapshotclient "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeConfigFlagName is the CLI flag name for kubeconfig path.
var KubeConfigFlagName = "kubeconfig"

// GetKubeConfig loads a *rest.Config from a kubeconfig file or in-cluster credentials.
//
// kubecfgPath, when non-empty, selects the kubeconfig file passed to clientcmd.BuildConfigFromFlags.
// When empty, rest.InClusterConfig is used (typical for Pods with a service account).
// It returns the REST config or a non-nil error.
func GetKubeConfig(kubecfgPath string) (*rest.Config, error) {
	var config *rest.Config
	var err error

	if kubecfgPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubecfgPath)
		if err != nil {
			return nil, fmt.Errorf("build kube config from flags: %w", err)
		}

		return config, nil
	}

	config, err = rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("get in-cluster kube config: %w", err)
	}

	return config, nil
}

// NewKubeClient constructs a kubernetes.Interface from config (core API clients).
//
// config must be non-nil and valid for the target cluster.
// It returns the interface or a non-nil error.
func NewKubeClient(config *rest.Config) (clientset.Interface, error) {
	client, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return client, nil
}

// NewSnapshotClient builds a clientset for snapshot.storage.k8s.io resources.
//
// config must be valid for the apiserver. Errors from NewForConfig indicate client construction failure
// (distinct from CRDs missing at runtime).
// It returns snapshotclient.Interface or a non-nil error.
func NewSnapshotClient(config *rest.Config) (snapshotclient.Interface, error) {
	client, err := snapshotclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create snapshot client: %w", err)
	}
	return client, nil
}
