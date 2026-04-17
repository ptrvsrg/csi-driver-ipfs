// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// BuildRESTConfig loads in-cluster config first, then falls back to KUBECONFIG/home config.
func BuildRESTConfig() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}
	return loadOutOfClusterConfig()
}

// NewClients builds strongly typed and dynamic Kubernetes clients.
func NewClients() (kubernetes.Interface, dynamic.Interface, snapshotv1.Interface, *rest.Config, error) {
	cfg, err := BuildRESTConfig()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("build kube config: %w", err)
	}
	core, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create core clientset: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create dynamic client: %w", err)
	}
	snap, err := snapshotv1.NewForConfig(cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create snapshot clientset: %w", err)
	}

	return core, dyn, snap, cfg, nil
}

func loadOutOfClusterConfig() (*rest.Config, error) {
	if kubeconfig := os.Getenv(clientcmd.RecommendedConfigPathEnvVar); kubeconfig != "" {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.Precedence = existingKubeconfigFiles(filepath.SplitList(kubeconfig))
		if len(rules.Precedence) > 0 {
			cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				rules,
				&clientcmd.ConfigOverrides{},
			).ClientConfig()
			if err == nil {
				return cfg, nil
			}
		}
	}

	info, err := os.Stat(clientcmd.RecommendedHomeFile)
	if err == nil && info.IsDir() {
		entries, readErr := os.ReadDir(clientcmd.RecommendedHomeFile)
		if readErr != nil {
			return nil, fmt.Errorf("read kubeconfig directory %s: %w", clientcmd.RecommendedHomeFile, readErr)
		}

		precedence := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			precedence = append(precedence, filepath.Join(clientcmd.RecommendedHomeFile, entry.Name()))
		}
		if len(precedence) == 0 {
			return nil, fmt.Errorf("kubeconfig directory %s contains no files", clientcmd.RecommendedHomeFile)
		}
		sort.Strings(precedence)

		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.Precedence = precedence
		cfg, cfgErr := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			rules,
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if cfgErr != nil {
			return nil, fmt.Errorf("load kubeconfig from directory %s: %w", clientcmd.RecommendedHomeFile, cfgErr)
		}
		return cfg, nil
	}

	return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
}

func existingKubeconfigFiles(candidates []string) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		out = append(out, candidate)
	}
	return out
}
