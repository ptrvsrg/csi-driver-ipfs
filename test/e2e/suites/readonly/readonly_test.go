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

package readonly_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ipfsEmptyDirCID = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"

var _ = Describe("Read-only volume", func() {
	It("mounts CID volume as read-only and rejects write operations", func(ctx SpecContext) {
		scName := fw.NewName("ro-sc")
		pvcName := fw.NewName("ro-pvc")
		podName := fw.NewName("ro-pod")

		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:   scName,
				Labels: fw.Labels(map[string]string{"case": "readonly-cid"}),
			},
			Provisioner:   fw.Config.DriverName,
			Parameters:    map[string]string{"cid": ipfsEmptyDirCID},
			ReclaimPolicy: ptr(corev1.PersistentVolumeReclaimDelete),
		}
		Expect(fw.CreateStorageClass(ctx, sc)).To(Succeed())

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: fw.Config.Namespace,
				Labels:    fw.Labels(map[string]string{"case": "readonly-cid"}),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
				StorageClassName: &scName,
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
				Labels:    fw.Labels(map[string]string{"case": "readonly-cid"}),
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:    "readonly",
						Image:   "busybox:1.36",
						Command: []string{"sh", "-c", "ls /data; if touch /data/x 2>/dev/null; then exit 1; fi; exit 0"},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "vol", MountPath: "/data", ReadOnly: true},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "vol",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcName,
								ReadOnly:  true,
							},
						},
					},
				},
			},
		}
		Expect(fw.CreatePod(ctx, pod)).To(Succeed())
		Expect(fw.WaitForPodSuccess(ctx, fw.Config.Namespace, podName)).To(Succeed())
	})
})

func ptr[T any](v T) *T { return &v }
