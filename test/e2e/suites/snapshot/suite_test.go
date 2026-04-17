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
	"context"
	"testing"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/ptrvsrg/csi-driver-ipfs/test/e2e/internal/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var fw *framework.Framework

func TestSnapshotSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Snapshot Suite")
}

var _ = BeforeSuite(func() {
	var err error
	fw, err = framework.New("snapshot")
	Expect(err).NotTo(HaveOccurred())

	if !fw.MustRun() {
		ginkgo.Skip("E2E disabled: set E2E_RUN=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), fw.Config.Timeout)
	defer cancel()

	Expect(fw.EnsureNamespace(ctx)).To(Succeed())
	Expect(fw.RequireStorageClass(ctx)).To(Succeed())
	if !fw.SnapshotAPISupported(ctx) {
		ginkgo.Skip("snapshot API is unavailable in target cluster")
	}
})

var _ = AfterSuite(func() {
	if fw == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	Expect(fw.Cleanup(ctx)).To(Succeed())
})

func ensureSnapshotClass(ctx context.Context, name string) {
	_, err := fw.Snapshots.SnapshotV1().VolumeSnapshotClasses().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return
	}

	vsc := &snapshotv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: fw.Labels(map[string]string{"case": "snapshot-class"}),
		},
		Driver:         fw.Config.DriverName,
		DeletionPolicy: snapshotv1.VolumeSnapshotContentDelete,
	}
	Expect(fw.CreateVolumeSnapshotClass(ctx, vsc)).To(Succeed())
}
