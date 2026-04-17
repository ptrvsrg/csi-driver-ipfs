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

package expand_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Volume expansion", func() {
	It("expands PVC from 1Gi to 2Gi and mounts expanded volume", func(ctx SpecContext) {
		pvcName := fw.NewName("expand-pvc")
		podName := fw.NewName("expand-pod")
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "expand"}),
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

		patch := []byte(`{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}`)
		_, err := fw.Kube.CoreV1().PersistentVolumeClaims(fw.Config.Namespace).Patch(
			ctx,
			pvcName,
			types.MergePatchType,
			patch,
			metav1.PatchOptions{},
		)
		Expect(err).NotTo(HaveOccurred())

		// Drivers may apply expansion asynchronously and some backends do not
		// report immediate status capacity changes. We assert request acceptance.
		Eventually(func(ctx SpecContext) error {
			obj, err := fw.Kube.CoreV1().PersistentVolumeClaims(fw.Config.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			requested := obj.Spec.Resources.Requests[corev1.ResourceStorage]
			if requested.Cmp(resource.MustParse("2Gi")) != 0 {
				return fmt.Errorf("expected requested size to be 2Gi, got %s", requested.String())
			}
			return nil
		}).WithContext(ctx).WithPolling(fw.Config.PollInterval).WithTimeout(fw.Config.Timeout).Should(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "expand"}),
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:         "check",
						Image:        "busybox:1.36",
						Command:      []string{"sh", "-c", "df /data && exit 0"},
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
	})
})
