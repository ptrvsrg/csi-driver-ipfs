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

// Package driver implements the CSI Identity, Controller, and Node services for IPFS-backed
// volumes, using Kubernetes PersistentVolume data (VolumeStore) and optional snapshot metadata.
package driver

import (
	"errors"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/lock"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/ipfs"
	"github.com/rs/zerolog"
)

const (
	DefaultDriverName = "ipfs.csi.ptrvsrg.github.io"

	// CSI volume context / PV attribute keys (must match values returned in CreateVolume).
	paramCID      = "cid"
	paramPinning  = "pinning"
	paramMFSPath  = "mfspath"
	paramReadOnly = "readonly"

	pinFailurePolicyStrict     = "strict"
	pinFailurePolicyBestEffort = "best-effort"
)

var (
	ErrEmptyDriverName         = errors.New("driver name is required")
	ErrEmptyNodeID             = errors.New("node ID is required")
	ErrInvalidPinFailurePolicy = errors.New("invalid pin failure policy")
)

// Config holds static settings for a single Driver instance.
type Config struct {
	// DriverName is the CSI driver name (e.g. StorageClass provisioner and CSI plugin name).
	DriverName string
	// NodeID identifies this kubelet node for CSI Node RPCs.
	NodeID string
	// Version is the driver build/version string reported in GetPluginInfo.
	Version string
	// MountDir is the base directory for per-volume bind mounts when no staging path is used.
	MountDir string
	// MFSRoot is the IPFS MFS path prefix under which read-write volumes are created by default.
	MFSRoot string
	// PinFailurePolicy controls CreateVolume behavior when pinning a CID fails.
	// Supported values: "strict" (return error), "best-effort" (log warning and continue).
	PinFailurePolicy string
}

// Driver implements csi.IdentityServer, csi.ControllerServer, and csi.NodeServer.
// Volume and snapshot metadata are resolved from Kubernetes via VolumeStore and SnapshotStore
// (informer-backed listers), not from in-memory state.
type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer
	csi.UnimplementedControllerServer
	csi.UnimplementedGroupControllerServer

	logger        zerolog.Logger
	cfg           *Config
	ipfsClient    ipfs.Client
	volumeStore   store.VolumeStore
	snapshotStore store.SnapshotStore // nil if snapshot CRD not installed
	volumeLocks   *lock.VolumeLocks
}

// NewDriver constructs a Driver that uses ipfsClient for IPFS operations and volumeStore to read
// PV-derived volume attributes. snapshotStore may be nil when the VolumeSnapshot API is unavailable.
//
// logger names the component in structured logs. cfg must set non-empty DriverName and NodeID or
// NewDriver returns ErrEmptyDriverName or ErrEmptyNodeID.
//
// On success it returns a ready-to-register *Driver; otherwise it returns a non-nil error.
func NewDriver(
	logger zerolog.Logger, cfg *Config, ipfsClient ipfs.Client, volumeStore store.VolumeStore,
	snapshotStore store.SnapshotStore,
) (*Driver, error) {
	if cfg.DriverName == "" {
		return nil, ErrEmptyDriverName
	}

	if cfg.NodeID == "" {
		return nil, ErrEmptyNodeID
	}

	if cfg.PinFailurePolicy == "" {
		cfg.PinFailurePolicy = pinFailurePolicyStrict
	}

	if cfg.PinFailurePolicy != pinFailurePolicyStrict && cfg.PinFailurePolicy != pinFailurePolicyBestEffort {
		return nil, ErrInvalidPinFailurePolicy
	}

	return &Driver{
		logger:        logger,
		cfg:           cfg,
		ipfsClient:    ipfsClient,
		volumeStore:   volumeStore,
		snapshotStore: snapshotStore,
		volumeLocks:   lock.NewVolumeLocks(),
	}, nil
}
