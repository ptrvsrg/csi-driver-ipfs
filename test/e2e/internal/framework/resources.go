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
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (f *Framework) CreateStorageClass(ctx context.Context, sc *storagev1.StorageClass) error {
	f.logOperation("storageclass create start", "name", sc.Name)
	err := f.withRetries(
		ctx, "storageclass create", func(attemptCtx context.Context) error {
			_, err := f.Kube.StorageV1().StorageClasses().Create(attemptCtx, sc, metav1.CreateOptions{})
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		},
	)
	if err != nil {
		f.logOperation("storageclass create failed", "name", sc.Name, "error", err)
		return fmt.Errorf("create storageclass %s: %w", sc.Name, err)
	}
	f.logOperation("storageclass create done", "name", sc.Name)

	f.RegisterCleanup(
		func(ctx context.Context) error {
			err := f.Kube.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		},
	)
	return nil
}

func (f *Framework) CreatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	f.logOperation("pvc create start", "namespace", pvc.Namespace, "name", pvc.Name)
	err := f.withRetries(
		ctx, "pvc create", func(attemptCtx context.Context) error {
			_, err := f.Kube.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(
				attemptCtx,
				pvc,
				metav1.CreateOptions{},
			)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		},
	)
	if err != nil {
		f.logOperation("pvc create failed", "namespace", pvc.Namespace, "name", pvc.Name, "error", err)
		return fmt.Errorf("create pvc %s/%s: %w", pvc.Namespace, pvc.Name, err)
	}
	f.logOperation("pvc create done", "namespace", pvc.Namespace, "name", pvc.Name)

	f.RegisterCleanup(
		func(ctx context.Context) error {
			err := f.Kube.CoreV1().PersistentVolumeClaims(pvc.Namespace).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		},
	)
	return nil
}

func (f *Framework) WaitForPVCBound(ctx context.Context, namespace, name string) error {
	f.logOperation("pvc wait bound start", "namespace", namespace, "name", name)
	err := f.withRetries(
		ctx, "pvc wait bound", func(attemptCtx context.Context) error {
			return wait.PollUntilContextTimeout(
				attemptCtx,
				f.Config.PollInterval,
				f.Config.Timeout,
				true,
				func(pollCtx context.Context) (bool, error) {
					pvc, err := f.Kube.CoreV1().PersistentVolumeClaims(namespace).Get(
						pollCtx,
						name,
						metav1.GetOptions{},
					)
					if err != nil {
						return false, err
					}
					return pvc.Status.Phase == corev1.ClaimBound, nil
				},
			)
		},
	)
	if err != nil {
		f.logOperation("pvc wait bound failed", "namespace", namespace, "name", name, "error", err)
		return err
	}
	f.logOperation("pvc wait bound done", "namespace", namespace, "name", name)
	return nil
}

func (f *Framework) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	f.logOperation("pod create start", "namespace", pod.Namespace, "name", pod.Name)
	err := f.withRetries(
		ctx, "pod create", func(attemptCtx context.Context) error {
			_, err := f.Kube.CoreV1().Pods(pod.Namespace).Create(attemptCtx, pod, metav1.CreateOptions{})
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		},
	)
	if err != nil {
		f.logOperation("pod create failed", "namespace", pod.Namespace, "name", pod.Name, "error", err)
		return fmt.Errorf("create pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	f.logOperation("pod create done", "namespace", pod.Namespace, "name", pod.Name)

	f.RegisterCleanup(
		func(ctx context.Context) error {
			err := f.Kube.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		},
	)
	return nil
}

func (f *Framework) WaitForPodSuccess(ctx context.Context, namespace, name string) error {
	f.logOperation("pod wait success start", "namespace", namespace, "name", name)
	var lastReason string
	err := f.withRetries(
		ctx, "pod wait success", func(attemptCtx context.Context) error {
			return wait.PollUntilContextTimeout(
				attemptCtx,
				f.Config.PollInterval,
				f.Config.Timeout,
				true,
				func(pollCtx context.Context) (bool, error) {
					pod, err := f.Kube.CoreV1().Pods(namespace).Get(pollCtx, name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if pod.Status.Phase == corev1.PodSucceeded {
						return true, nil
					}
					if pod.Status.Phase == corev1.PodFailed {
						lastReason = pod.Status.Reason
						return true, fmt.Errorf("pod %s/%s failed: %s", namespace, name, pod.Status.Reason)
					}
					return false, nil
				},
			)
		},
	)
	if err != nil && lastReason != "" {
		f.logOperation(
			"pod wait success failed",
			"namespace",
			namespace,
			"name",
			name,
			"reason",
			lastReason,
			"error",
			err,
		)
		return fmt.Errorf("wait pod success: %w", err)
	}
	if err != nil {
		f.logOperation("pod wait success failed", "namespace", namespace, "name", name, "error", err)
		return err
	}
	f.logOperation("pod wait success done", "namespace", namespace, "name", name)
	return nil
}

func (f *Framework) DeletePodNow(ctx context.Context, namespace, name string) error {
	f.logOperation("pod delete start", "namespace", namespace, "name", name)
	err := f.Kube.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		f.logOperation("pod delete not found", "namespace", namespace, "name", name)
		return nil
	}
	if err != nil {
		f.logOperation("pod delete failed", "namespace", namespace, "name", name, "error", err)
		return err
	}
	f.logOperation("pod delete done", "namespace", namespace, "name", name)
	return err
}

func (f *Framework) WaitForPodDeleted(ctx context.Context, namespace, name string) error {
	f.logOperation("pod wait deleted start", "namespace", namespace, "name", name)
	err := f.withRetries(
		ctx, "pod wait deleted", func(attemptCtx context.Context) error {
			return wait.PollUntilContextTimeout(
				attemptCtx,
				f.Config.PollInterval,
				f.Config.Timeout,
				true,
				func(pollCtx context.Context) (bool, error) {
					_, err := f.Kube.CoreV1().Pods(namespace).Get(pollCtx, name, metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				},
			)
		},
	)
	if err != nil {
		f.logOperation("pod wait deleted failed", "namespace", namespace, "name", name, "error", err)
		return err
	}
	f.logOperation("pod wait deleted done", "namespace", namespace, "name", name)
	return nil
}

func (f *Framework) WaitForPVDeleted(ctx context.Context, pvName string) error {
	f.logOperation("pv wait deleted start", "name", pvName)
	err := f.withRetries(
		ctx, "pv wait deleted", func(attemptCtx context.Context) error {
			return wait.PollUntilContextTimeout(
				attemptCtx,
				f.Config.PollInterval,
				f.Config.Timeout,
				true,
				func(pollCtx context.Context) (bool, error) {
					_, err := f.Kube.CoreV1().PersistentVolumes().Get(pollCtx, pvName, metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				},
			)
		},
	)
	if err != nil {
		f.logOperation("pv wait deleted failed", "name", pvName, "error", err)
		return err
	}
	f.logOperation("pv wait deleted done", "name", pvName)
	return nil
}

func (f *Framework) CreateVolumeSnapshotClass(
	ctx context.Context,
	vsc *snapshotv1.VolumeSnapshotClass,
) error {
	f.logOperation("snapshot class create start", "name", vsc.Name)
	err := f.withRetries(
		ctx, "snapshot class create", func(attemptCtx context.Context) error {
			_, err := f.Snapshots.SnapshotV1().VolumeSnapshotClasses().Create(attemptCtx, vsc, metav1.CreateOptions{})
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		},
	)
	if err != nil {
		f.logOperation("snapshot class create failed", "name", vsc.Name, "error", err)
		return fmt.Errorf("create VolumeSnapshotClass %s: %w", vsc.Name, err)
	}
	f.logOperation("snapshot class create done", "name", vsc.Name)
	f.RegisterCleanup(
		func(ctx context.Context) error {
			err := f.Snapshots.SnapshotV1().VolumeSnapshotClasses().Delete(ctx, vsc.Name, metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		},
	)
	return nil
}

func (f *Framework) CreateVolumeSnapshot(
	ctx context.Context,
	vs *snapshotv1.VolumeSnapshot,
) error {
	f.logOperation("snapshot create start", "namespace", vs.Namespace, "name", vs.Name)
	err := f.withRetries(
		ctx, "snapshot create", func(attemptCtx context.Context) error {
			_, err := f.Snapshots.SnapshotV1().VolumeSnapshots(vs.Namespace).Create(
				attemptCtx,
				vs,
				metav1.CreateOptions{},
			)
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		},
	)
	if err != nil {
		f.logOperation("snapshot create failed", "namespace", vs.Namespace, "name", vs.Name, "error", err)
		return fmt.Errorf("create VolumeSnapshot %s/%s: %w", vs.Namespace, vs.Name, err)
	}
	f.logOperation("snapshot create done", "namespace", vs.Namespace, "name", vs.Name)
	f.RegisterCleanup(
		func(ctx context.Context) error {
			err := f.Snapshots.SnapshotV1().VolumeSnapshots(vs.Namespace).Delete(ctx, vs.Name, metav1.DeleteOptions{})
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		},
	)
	return nil
}

func (f *Framework) WaitForSnapshotReady(ctx context.Context, namespace, name string) error {
	f.logOperation("snapshot wait ready start", "namespace", namespace, "name", name)
	err := f.withRetries(
		ctx, "snapshot wait ready", func(attemptCtx context.Context) error {
			return wait.PollUntilContextTimeout(
				attemptCtx,
				3*time.Second,
				f.Config.Timeout,
				true,
				func(pollCtx context.Context) (bool, error) {
					vs, err := f.Snapshots.SnapshotV1().VolumeSnapshots(namespace).Get(
						pollCtx,
						name,
						metav1.GetOptions{},
					)
					if err != nil {
						return false, err
					}
					if vs.Status == nil || vs.Status.ReadyToUse == nil {
						return false, nil
					}
					return *vs.Status.ReadyToUse, nil
				},
			)
		},
	)
	if err != nil {
		f.logOperation("snapshot wait ready failed", "namespace", namespace, "name", name, "error", err)
		return err
	}
	f.logOperation("snapshot wait ready done", "namespace", namespace, "name", name)
	return nil
}
