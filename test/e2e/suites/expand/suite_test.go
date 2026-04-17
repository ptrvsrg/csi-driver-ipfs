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
	"context"
	"testing"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/ptrvsrg/csi-driver-ipfs/test/e2e/internal/framework"
)

var fw *framework.Framework

func TestExpandSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Expand Suite")
}

var _ = BeforeSuite(func() {
	var err error
	fw, err = framework.New("expand")
	Expect(err).NotTo(HaveOccurred())

	if !fw.MustRun() {
		ginkgo.Skip("E2E disabled: set E2E_RUN=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), fw.Config.Timeout)
	defer cancel()

	Expect(fw.EnsureNamespace(ctx)).To(Succeed())
	Expect(fw.RequireStorageClass(ctx)).To(Succeed())
})

var _ = AfterSuite(func() {
	if fw == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	Expect(fw.Cleanup(ctx)).To(Succeed())
})
