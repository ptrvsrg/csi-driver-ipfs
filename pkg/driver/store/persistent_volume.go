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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
)

var (
	// ErrPVNotFound is returned when a PersistentVolume is not found.
	ErrPVNotFound = errors.New("persistent volume not found")
)

// VolumeAttributes aggregates CSI metadata derived from a PersistentVolume.
type VolumeAttributes struct {
	// VolumeID is the CSI volume handle (typically equal to PV name or spec.csi.volumeHandle).
	VolumeID string
	// Capacity is the PV storage capacity in bytes when present.
	Capacity int64
	// Attributes mirrors spec.csi.volumeAttributes (cid, mfspath, etc.).
	Attributes map[string]string
}

// VolumeStore reads PersistentVolume-backed state for controller and node operations.
type VolumeStore interface {
	// GetVolumeAttributes returns attributes for the given CSI volume ID, or ErrPVNotFound.
	GetVolumeAttributes(ctx context.Context, volumeID string) (*VolumeAttributes, error)
	// ListVolumes returns all volumes provisioned by driverName (matched on spec.csi.driver).
	ListVolumes(ctx context.Context, driverName string) ([]*VolumeAttributes, error)
}

// pvStore adapts a PV Lister to VolumeStore.
type pvStore struct {
	lister corelisters.PersistentVolumeLister
}

// NewVolumeStoreFromPVLister wraps lister as a VolumeStore that resolves volumes by volumeHandle.
//
// lister must be backed by a synced informer for consistent reads.
// It returns a non-nil VolumeStore implementation.
func NewVolumeStoreFromPVLister(lister corelisters.PersistentVolumeLister) VolumeStore {
	return &pvStore{lister: lister}
}

func (s *pvStore) GetVolumeAttributes(_ context.Context, volumeID string) (*VolumeAttributes, error) {
	pv, err := s.lister.Get(volumeID)
	if err == nil && pv.Spec.CSI != nil && pv.Spec.CSI.VolumeHandle == volumeID {
		return pvToAttributes(pv)
	}

	all, err := s.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list PVs: %w", err)
	}

	for _, pv := range all {
		if pv.Spec.CSI != nil && pv.Spec.CSI.VolumeHandle == volumeID {
			return pvToAttributes(pv)
		}
	}

	return nil, fmt.Errorf("%w: volume %s", ErrPVNotFound, volumeID)
}

func (s *pvStore) ListVolumes(_ context.Context, driverName string) ([]*VolumeAttributes, error) {
	all, err := s.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("list PVs: %w", err)
	}

	var out []*VolumeAttributes
	for _, pv := range all {
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != driverName {
			continue
		}

		attrs, err := pvToAttributes(pv)
		if err != nil {
			continue
		}

		out = append(out, attrs)
	}

	return out, nil
}

func pvToAttributes(pv *corev1.PersistentVolume) (*VolumeAttributes, error) {
	if pv.Spec.CSI == nil {
		return nil, errors.New("PV has no CSI spec")
	}

	volID := pv.Spec.CSI.VolumeHandle
	if volID == "" {
		volID = pv.Name
	}

	var capacity int64
	if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = q.Value()
	}

	attrs := make(map[string]string)
	for k, v := range pv.Spec.CSI.VolumeAttributes {
		attrs[k] = v
	}

	return &VolumeAttributes{
		VolumeID:   volID,
		Capacity:   capacity,
		Attributes: attrs,
	}, nil
}
