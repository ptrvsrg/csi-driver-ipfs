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

package snapshot_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
)

var _ = Describe("Snapshot flows", Ordered, func() {
	It("creates snapshot from source PVC and waits for readyToUse", SpecTimeout(10*time.Minute), func(ctx SpecContext) {
		pvcName := fw.NewName("snap-src-pvc")
		podName := fw.NewName("snap-write-pod")
		vsClassName := fw.NewName("snap-class")
		vsName := fw.NewName("snap")

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-create"}),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &fw.Config.StorageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}
		Expect(fw.CreatePVC(ctx, pvc)).To(Succeed())
		Expect(fw.WaitForPVCBound(ctx, fw.Config.Namespace, pvcName)).To(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-create"}),
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:         "write",
						Image:        "busybox:1.36",
						Command:      []string{"sh", "-c", "echo snapshot-me > /data/foo && sync && exit 0"},
						VolumeMounts: []corev1.VolumeMount{{Name: "vol", MountPath: "/data"}},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "vol",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
						},
					},
				},
			},
		}
		Expect(fw.CreatePod(ctx, pod)).To(Succeed())
		Expect(fw.WaitForPodSuccess(ctx, fw.Config.Namespace, podName)).To(Succeed())
		Expect(fw.DeletePodNow(ctx, fw.Config.Namespace, podName)).To(Succeed())

		ensureSnapshotClass(ctx, vsClassName)

		vs := &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vsName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-create"}),
			},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &pvcName,
				},
				VolumeSnapshotClassName: &vsClassName,
			},
		}
		Expect(fw.CreateVolumeSnapshot(ctx, vs)).To(Succeed())
		Expect(fw.WaitForSnapshotReady(ctx, fw.Config.Namespace, vsName)).To(Succeed())
	})

	It("restores PVC from snapshot and verifies data", SpecTimeout(10*time.Minute), func(ctx SpecContext) {
		srcPVCName := fw.NewName("restore-src-pvc")
		dstPVCName := fw.NewName("restore-dst-pvc")
		writePodName := fw.NewName("restore-write-pod")
		readPodName := fw.NewName("restore-read-pod")
		vsClassName := fw.NewName("restore-class")
		vsName := fw.NewName("restore-snap")

		srcPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      srcPVCName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-restore"}),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &fw.Config.StorageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}
		Expect(fw.CreatePVC(ctx, srcPVC)).To(Succeed())
		Expect(fw.WaitForPVCBound(ctx, fw.Config.Namespace, srcPVCName)).To(Succeed())

		writePod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      writePodName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-restore"}),
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:         "write",
						Image:        "busybox:1.36",
						Command:      []string{"sh", "-c", "echo restore-me > /data/source.txt && sync && exit 0"},
						VolumeMounts: []corev1.VolumeMount{{Name: "vol", MountPath: "/data"}},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "vol",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: srcPVCName},
						},
					},
				},
			},
		}
		Expect(fw.CreatePod(ctx, writePod)).To(Succeed())
		Expect(fw.WaitForPodSuccess(ctx, fw.Config.Namespace, writePodName)).To(Succeed())
		Expect(fw.DeletePodNow(ctx, fw.Config.Namespace, writePodName)).To(Succeed())

		ensureSnapshotClass(ctx, vsClassName)
		vs := &snapshotv1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vsName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-restore"}),
			},
			Spec: snapshotv1.VolumeSnapshotSpec{
				Source: snapshotv1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &srcPVCName,
				},
				VolumeSnapshotClassName: &vsClassName,
			},
		}
		Expect(fw.CreateVolumeSnapshot(ctx, vs)).To(Succeed())
		Expect(fw.WaitForSnapshotReady(ctx, fw.Config.Namespace, vsName)).To(Succeed())

		dstPVC := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dstPVCName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-restore"}),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &fw.Config.StorageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				DataSource: &corev1.TypedLocalObjectReference{
					APIGroup: ptr("snapshot.storage.k8s.io"),
					Kind:     "VolumeSnapshot",
					Name:     vsName,
				},
			},
		}
		Expect(fw.CreatePVC(ctx, dstPVC)).To(Succeed())
		Expect(fw.WaitForPVCBound(ctx, fw.Config.Namespace, dstPVCName)).To(Succeed())

		readPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      readPodName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "snapshot-restore"}),
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:         "read",
						Image:        "busybox:1.36",
						Command:      []string{"sh", "-c", "cat /data/source.txt | grep -q restore-me && exit 0"},
						VolumeMounts: []corev1.VolumeMount{{Name: "vol", MountPath: "/data"}},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "vol",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: dstPVCName},
						},
					},
				},
			},
		}
		Expect(fw.CreatePod(ctx, readPod)).To(Succeed())
		Expect(fw.WaitForPodSuccess(ctx, fw.Config.Namespace, readPodName)).To(Succeed())
	})
})

func ptr[T any](v T) *T { return &v }
