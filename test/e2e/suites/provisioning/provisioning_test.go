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

package provisioning_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe(
	"Dynamic provisioning", Ordered, func() {
		It(
			"provisions volume, mounts it and allows write/read",
			SpecTimeout(10*time.Minute),
			func(ctx SpecContext) {
				pvcName := fw.NewName("dyn-pvc")
				podName := fw.NewName("dyn-pod")

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: fw.Config.Namespace,
						Labels:    fw.Labels(map[string]string{"case": "dynamic-single"}),
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
						Labels:    fw.Labels(map[string]string{"case": "dynamic-single"}),
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:  "writer",
								Image: "busybox:1.36",
								Command: []string{
									"sh",
									"-c",
									"echo e2e-hello-from-ipfs > /data/hello && cat /data/hello && exit 0",
								},
								VolumeMounts: []corev1.VolumeMount{
									{Name: "vol", MountPath: "/data"},
								},
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
			},
		)

		It(
			"persists data across sequential pods", SpecTimeout(10*time.Minute), func(ctx SpecContext) {
				pvcName := fw.NewName("shared-pvc")
				pod1Name := fw.NewName("shared-pod1")
				pod2Name := fw.NewName("shared-pod2")

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: fw.Config.Namespace,
						Labels:    fw.Labels(map[string]string{"case": "dynamic-two-pods"}),
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

				pod1 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod1Name,
						Namespace: fw.Config.Namespace,
						Labels:    fw.Labels(map[string]string{"case": "dynamic-two-pods", "pod-phase": "1"}),
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:         "write",
								Image:        "busybox:1.36",
								Command:      []string{"sh", "-c", "echo pod1 > /data/pod1.txt && exit 0"},
								VolumeMounts: []corev1.VolumeMount{{Name: "vol", MountPath: "/data"}},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "vol",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvcName,
									},
								},
							},
						},
					},
				}
				Expect(fw.CreatePod(ctx, pod1)).To(Succeed())
				Expect(fw.WaitForPodSuccess(ctx, fw.Config.Namespace, pod1Name)).To(Succeed())
				Expect(fw.DeletePodNow(ctx, fw.Config.Namespace, pod1Name)).To(Succeed())
				Expect(fw.WaitForPodDeleted(ctx, fw.Config.Namespace, pod1Name)).To(Succeed())

				pod2 := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod2Name,
						Namespace: fw.Config.Namespace,
						Labels:    fw.Labels(map[string]string{"case": "dynamic-two-pods", "pod-phase": "2"}),
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{
							{
								Name:  "readwrite",
								Image: "busybox:1.36",
								Command: []string{
									"sh",
									"-c",
									"cat /data/pod1.txt | grep -q pod1 && echo pod2 > /data/pod2.txt && exit 0",
								},
								VolumeMounts: []corev1.VolumeMount{{Name: "vol", MountPath: "/data"}},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "vol",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: pvcName,
									},
								},
							},
						},
					},
				}
				Expect(fw.CreatePod(ctx, pod2)).To(Succeed())
				Expect(fw.WaitForPodSuccess(ctx, fw.Config.Namespace, pod2Name)).To(Succeed())
			},
		)

		It(
			"deletes bound PV after PVC delete when reclaim policy is Delete",
			SpecTimeout(10*time.Minute),
			func(ctx SpecContext) {
				pvcName := fw.NewName("reclaim-pvc")
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: fw.Config.Namespace,
						Labels:    fw.Labels(map[string]string{"case": "reclaim-delete"}),
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

				pvcObj, err := fw.Kube.CoreV1().PersistentVolumeClaims(fw.Config.Namespace).Get(
					ctx,
					pvcName,
					metav1.GetOptions{},
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(pvcObj.Spec.VolumeName).NotTo(BeEmpty())
				pvName := pvcObj.Spec.VolumeName

				Expect(
					fw.Kube.CoreV1().PersistentVolumeClaims(fw.Config.Namespace).Delete(
						ctx,
						pvcName,
						metav1.DeleteOptions{},
					),
				).To(Succeed())
				Eventually(
					func(ctx context.Context) error {
						_, err := fw.Kube.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
						if apierrors.IsNotFound(err) {
							return nil
						}
						if err != nil {
							return err
						}
						return fmt.Errorf("pv %s still exists", pvName)
					},
				).WithContext(ctx).WithPolling(fw.Config.PollInterval).WithTimeout(fw.Config.Timeout).Should(Succeed())
			},
		)
	},
)
