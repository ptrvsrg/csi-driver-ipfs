// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import "testing"

func TestFromPtr(t *testing.T) {
	t.Parallel()

	v := 42
	tests := []struct {
		name string
		p    *int
		want int
	}{
		{"nil returns zero", nil, 0},
		{"deref", &v, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := FromPtr(tt.p); got != tt.want {
				t.Fatalf("FromPtr() = %v, want %v", got, tt.want)
			}
		})
	}
}
