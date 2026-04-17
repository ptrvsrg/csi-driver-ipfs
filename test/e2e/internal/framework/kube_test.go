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
	"os"
	"path/filepath"
	"testing"
)

func TestExistingKubeconfigFiles_FiltersMissingAndDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fileA := filepath.Join(dir, "config-a")
	fileB := filepath.Join(dir, "config-b")
	if err := os.WriteFile(fileA, []byte("a"), 0600); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0600); err != nil {
		t.Fatalf("write fileB: %v", err)
	}

	got := existingKubeconfigFiles([]string{
		fileA,
		filepath.Join(dir, "missing"),
		dir,
		"",
		fileB,
	})

	if len(got) != 2 {
		t.Fatalf("existingKubeconfigFiles() len = %d, want 2", len(got))
	}
	if got[0] != fileA || got[1] != fileB {
		t.Fatalf("existingKubeconfigFiles() = %v, want [%q %q]", got, fileA, fileB)
	}
}
