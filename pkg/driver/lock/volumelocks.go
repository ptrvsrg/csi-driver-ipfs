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

package lock

import "sync"

// VolumeLocks ensures only one controller operation runs per volume at a time.
// Prevents races when CreateVolume or DeleteVolume is retried or called concurrently
// for the same volume (e.g. duplicate requests, timeouts).
type VolumeLocks struct {
	mu    sync.Mutex
	locks map[string]struct{}
}

// NewVolumeLocks returns an empty VolumeLocks map.
func NewVolumeLocks() *VolumeLocks {
	return &VolumeLocks{
		locks: make(map[string]struct{}),
	}
}

// TryAcquire attempts to take the exclusive lock for key (volume name or ID).
//
// It returns true if the lock was acquired, or false if key is already locked.
func (vl *VolumeLocks) TryAcquire(key string) bool {
	vl.mu.Lock()
	defer vl.mu.Unlock()
	if _, held := vl.locks[key]; held {
		return false
	}
	vl.locks[key] = struct{}{}
	return true
}

// Release drops the lock for key if held; it is safe if key was not locked.
func (vl *VolumeLocks) Release(key string) {
	vl.mu.Lock()
	defer vl.mu.Unlock()
	delete(vl.locks, key)
}
