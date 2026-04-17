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
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DumpDiagnostics logs key cluster state to simplify failed E2E analysis.
func (f *Framework) DumpDiagnostics(ctx context.Context, namespace string) error {
	f.logOperation("diagnostics start", "namespace", namespace)

	if err := f.logPods(ctx, namespace); err != nil {
		f.logOperation("diagnostics pods failed", "error", err)
	}
	if err := f.logPVCs(ctx, namespace); err != nil {
		f.logOperation("diagnostics pvcs failed", "error", err)
	}
	if err := f.logPVs(ctx); err != nil {
		f.logOperation("diagnostics pvs failed", "error", err)
	}
	if err := f.logSnapshots(ctx, namespace); err != nil {
		f.logOperation("diagnostics snapshots failed", "error", err)
	}
	if err := f.logEvents(ctx, namespace); err != nil {
		f.logOperation("diagnostics events failed", "error", err)
	}
	if err := f.logControllerPods(ctx); err != nil {
		f.logOperation("diagnostics controllers failed", "error", err)
	}

	f.logOperation("diagnostics finished", "namespace", namespace)
	return nil
}

func (f *Framework) logPods(ctx context.Context, namespace string) error {
	pods, err := f.Kube.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	for _, pod := range pods.Items {
		f.logOperation(
			"diagnostics pod",
			"namespace", pod.Namespace,
			"name", pod.Name,
			"phase", pod.Status.Phase,
			"node", pod.Spec.NodeName,
			"reason", pod.Status.Reason,
		)
	}
	return nil
}

func (f *Framework) logPVCs(ctx context.Context, namespace string) error {
	pvcs, err := f.Kube.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pvcs: %w", err)
	}
	for _, pvc := range pvcs.Items {
		f.logOperation(
			"diagnostics pvc",
			"namespace", pvc.Namespace,
			"name", pvc.Name,
			"phase", pvc.Status.Phase,
			"volume", pvc.Spec.VolumeName,
		)
	}
	return nil
}

func (f *Framework) logPVs(ctx context.Context) error {
	pvs, err := f.Kube.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list pvs: %w", err)
	}
	for _, pv := range pvs.Items {
		claimNS := ""
		claimName := ""
		if pv.Spec.ClaimRef != nil {
			claimNS = pv.Spec.ClaimRef.Namespace
			claimName = pv.Spec.ClaimRef.Name
		}
		f.logOperation(
			"diagnostics pv",
			"name", pv.Name,
			"phase", pv.Status.Phase,
			"reclaim_policy", pv.Spec.PersistentVolumeReclaimPolicy,
			"claim_namespace", claimNS,
			"claim_name", claimName,
		)
	}
	return nil
}

func (f *Framework) logSnapshots(ctx context.Context, namespace string) error {
	vsList, err := f.Snapshots.SnapshotV1().VolumeSnapshots(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list volumesnapshots: %w", err)
	}
	for _, vs := range vsList.Items {
		ready := false
		if vs.Status != nil && vs.Status.ReadyToUse != nil {
			ready = *vs.Status.ReadyToUse
		}
		f.logOperation(
			"diagnostics volumesnapshot",
			"namespace", vs.Namespace,
			"name", vs.Name,
			"ready", ready,
			"content_name", valueOrEmpty(vs.Status != nil && vs.Status.BoundVolumeSnapshotContentName != nil, func() string {
				return *vs.Status.BoundVolumeSnapshotContentName
			}),
		)
	}

	vscList, err := f.Snapshots.SnapshotV1().VolumeSnapshotContents().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list volumesnapshotcontents: %w", err)
	}
	for _, vsc := range vscList.Items {
		f.logOperation(
			"diagnostics volumesnapshotcontent",
			"name", vsc.Name,
			"deletion_policy", vsc.Spec.DeletionPolicy,
			"snapshot_ref", valueOrEmpty(vsc.Spec.VolumeSnapshotRef.Name != "", func() string {
				return vsc.Spec.VolumeSnapshotRef.Namespace + "/" + vsc.Spec.VolumeSnapshotRef.Name
			}),
		)
	}
	return nil
}

func (f *Framework) logEvents(ctx context.Context, namespace string) error {
	events, err := f.Kube.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}

	sort.SliceStable(events.Items, func(i, j int) bool {
		return events.Items[i].LastTimestamp.Time.Before(events.Items[j].LastTimestamp.Time)
	})

	limit := 50
	if len(events.Items) < limit {
		limit = len(events.Items)
	}
	start := len(events.Items) - limit
	if start < 0 {
		start = 0
	}
	for _, e := range events.Items[start:] {
		msg := strings.TrimSpace(e.Message)
		f.logOperation(
			"diagnostics event",
			"type", e.Type,
			"reason", e.Reason,
			"obj", e.InvolvedObject.Kind+"/"+e.InvolvedObject.Name,
			"message", msg,
		)
	}
	return nil
}

func (f *Framework) logControllerPods(ctx context.Context) error {
	namespaces := []string{"default", "kube-system"}
	for _, ns := range namespaces {
		pods, err := f.Kube.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("list controller pods in %s: %w", ns, err)
		}
		for _, pod := range pods.Items {
			if !isControllerPod(pod.Name) {
				continue
			}
			for _, c := range pod.Spec.Containers {
				req := f.Kube.CoreV1().Pods(ns).GetLogs(pod.Name, &corev1.PodLogOptions{
					Container: c.Name,
					TailLines: int64Ptr(120),
				})
				raw, err := req.DoRaw(ctx)
				if err != nil {
					f.logOperation(
						"diagnostics controller log failed",
						"namespace", ns,
						"pod", pod.Name,
						"container", c.Name,
						"error", err,
					)
					continue
				}
				f.logOperation(
					"diagnostics controller log",
					"namespace", ns,
					"pod", pod.Name,
					"container", c.Name,
					"log_tail", string(raw),
				)
			}
		}
	}
	return nil
}

func int64Ptr(v int64) *int64 { return &v }

func isControllerPod(name string) bool {
	return strings.Contains(name, "csi-ipfs-controller") || strings.Contains(name, "snapshot-controller")
}

func valueOrEmpty(ok bool, producer func() string) string {
	if !ok {
		return ""
	}
	return producer()
}
