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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RequireStorageClass ensures default storage class for test run exists.
func (f *Framework) RequireStorageClass(ctx context.Context) error {
	f.logOperation("preflight storageclass check", "name", f.Config.StorageClass)
	_, err := f.Kube.StorageV1().StorageClasses().Get(ctx, f.Config.StorageClass, metav1.GetOptions{})
	if err != nil {
		f.logOperation("preflight storageclass failed", "name", f.Config.StorageClass, "error", err)
		return fmt.Errorf("storageclass %q is required for e2e: %w", f.Config.StorageClass, err)
	}
	f.logOperation("preflight storageclass ok", "name", f.Config.StorageClass)
	return nil
}

// SnapshotAPISupported reports if VolumeSnapshotClass CRD is installed.
func (f *Framework) SnapshotAPISupported(ctx context.Context) bool {
	f.logOperation("preflight snapshot api check")
	_, err := f.Snapshots.SnapshotV1().VolumeSnapshotClasses().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		f.logOperation("preflight snapshot api unavailable", "error", err)
		return false
	}
	f.logOperation("preflight snapshot api ok")
	return true
}

// EnsureNamespace verifies target namespace exists.
func (f *Framework) EnsureNamespace(ctx context.Context) error {
	f.logOperation("preflight namespace check", "namespace", f.Config.Namespace)
	_, err := f.Kube.CoreV1().Namespaces().Get(ctx, f.Config.Namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			f.logOperation("preflight namespace not found", "namespace", f.Config.Namespace)
			return fmt.Errorf("namespace %q not found", f.Config.Namespace)
		}
		f.logOperation("preflight namespace failed", "namespace", f.Config.Namespace, "error", err)
		return fmt.Errorf("get namespace %q: %w", f.Config.Namespace, err)
	}
	f.logOperation("preflight namespace ok", "namespace", f.Config.Namespace)
	return nil
}
