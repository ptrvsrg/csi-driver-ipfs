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

package store

import (
	"context"
	"fmt"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	snapshotlisters "github.com/kubernetes-csi/external-snapshotter/client/v8/listers/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// SnapshotEntry is a denormalized snapshot row derived from VolumeSnapshotContent status/spec.
type SnapshotEntry struct {
	// SnapshotID is the snapshot handle (typically the IPFS CID for this driver).
	SnapshotID string
	// SourceVolumeID is the CSI volume ID of the source volume when known.
	SourceVolumeID string
	// ReadyToUse mirrors status.readyToUse.
	ReadyToUse bool
	// CreationTime is Unix seconds when creation time is known, else zero.
	CreationTime int64
	// SizeBytes is restore size from status when set.
	SizeBytes int64
}

// SnapshotStore lists snapshots from VolumeSnapshotContent objects. Implementations may be nil when the API is absent.
type SnapshotStore interface {
	// ListSnapshots returns entries for driverName, optionally filtered by snapshotID and/or
	// sourceVolumeID (empty string disables that filter).
	ListSnapshots(ctx context.Context, driverName, snapshotID, sourceVolumeID string) ([]SnapshotEntry, error)
}

// vscStore adapts a VolumeSnapshotContent Lister to SnapshotStore.
type vscStore struct {
	lister snapshotlisters.VolumeSnapshotContentLister
}

// NewSnapshotStoreFromVSCLister wraps lister as a SnapshotStore.
//
// lister should come from a synced VolumeSnapshotContent informer. If lister is nil, nil is returned
// and callers must disable snapshot RPCs that require this store.
func NewSnapshotStoreFromVSCLister(lister snapshotlisters.VolumeSnapshotContentLister) SnapshotStore {
	if lister == nil {
		return nil
	}

	return &vscStore{lister: lister}
}

func (s *vscStore) ListSnapshots(
	_ context.Context, driverName, filterSnapshotID, filterSourceVolumeID string,
) ([]SnapshotEntry, error) {
	all, err := s.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list VolumeSnapshotContents: %w", err)
	}

	var out []SnapshotEntry
	for _, vsc := range all {
		entry, ok := vscToSnapshotEntry(vsc, driverName, filterSnapshotID, filterSourceVolumeID)
		if !ok {
			continue
		}

		out = append(out, entry)
	}

	return out, nil
}

func vscToSnapshotEntry(
	vsc *snapshotv1.VolumeSnapshotContent, driverName, filterSnapshotID, filterSourceVolumeID string,
) (SnapshotEntry, bool) {
	if vsc.Spec.Driver != driverName {
		return SnapshotEntry{}, false
	}

	sourceVol := ""
	if vsc.Spec.Source.VolumeHandle != nil {
		sourceVol = *vsc.Spec.Source.VolumeHandle
	}

	snapHandle := ""
	if vsc.Status != nil && vsc.Status.SnapshotHandle != nil {
		snapHandle = *vsc.Status.SnapshotHandle
	}

	if snapHandle == "" && vsc.Spec.Source.SnapshotHandle != nil {
		snapHandle = *vsc.Spec.Source.SnapshotHandle
	}

	if filterSnapshotID != "" && snapHandle != filterSnapshotID {
		return SnapshotEntry{}, false
	}

	if filterSourceVolumeID != "" && sourceVol != filterSourceVolumeID {
		return SnapshotEntry{}, false
	}

	ready := false
	if vsc.Status != nil && vsc.Status.ReadyToUse != nil {
		ready = *vsc.Status.ReadyToUse
	}

	var creationTime int64
	if vsc.Status != nil && vsc.Status.CreationTime != nil {
		creationTime = *vsc.Status.CreationTime / 1e9
	}

	var sizeBytes int64
	if vsc.Status != nil && vsc.Status.RestoreSize != nil {
		sizeBytes = *vsc.Status.RestoreSize
	}

	return SnapshotEntry{
		SnapshotID:     snapHandle,
		SourceVolumeID: sourceVol,
		ReadyToUse:     ready,
		CreationTime:   creationTime,
		SizeBytes:      sizeBytes,
	}, true
}
