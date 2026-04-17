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

package driver

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateVolume implements csi.ControllerServer. It provisions IPFS-backed volume state and returns
// volume context for the external provisioner to persist on the PersistentVolume (idempotent by name).
//
// ctx limits IPFS and locking work. req must set Name; Parameters may include cid, mfspath, pinning, etc.
// It returns CreateVolumeResponse with VolumeId, CapacityBytes, and VolumeContext, or a gRPC status error
// (InvalidArgument, NotFound, Internal, Aborted, ...).
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "volume name is required")
	}

	d.logger.Info().
		Str("name", name).
		Int("param_count", len(req.GetParameters())).
		Msg("create volume")

	params := req.GetParameters()
	if params == nil {
		params = make(map[string]string)
	}

	sizeBytes := parseCapacityFromRequest(req.GetCapacityRange(), defaultCapacityBytes)

	cid := params[paramCID]
	mfsPath := params[paramMFSPath]
	pinning := params[paramPinning] != "false"
	volumeID := fmt.Sprintf("ipfs-volume-%s", sanitizeVolumeID(name))

	snapshotSourceID, contentSourceErr := snapshotSourceID(req.GetVolumeContentSource())
	if contentSourceErr != nil {
		return nil, contentSourceErr
	}

	if snapshotSourceID != "" && cid != "" {
		return nil, status.Error(codes.InvalidArgument, "cid parameter cannot be combined with snapshot content source")
	}

	if acquired := d.volumeLocks.TryAcquire(volumeID); !acquired {
		d.logger.Debug().Str("id", volumeID).Msg("create volume: operation already in progress")
		return nil, status.Errorf(codes.Aborted, "an operation with the given volume id %q already exists", volumeID)
	}
	defer d.volumeLocks.Release(volumeID)

	// Build volume context from parameters; we will set cid, mfspath, readonly below.
	volumeContext := maps.Clone(params)
	readOnly := false
	var err error

	switch {
	case cid != "":
		// Read-only volume: content is identified by CID.
		readOnly = true
		d.logger.Debug().Str("cid", cid).Msg("volume type: read-only by CID")

		exists, err := d.ipfsClient.StatCID(ctx, cid)
		if err != nil {
			d.logger.Error().Err(err).Str("cid", cid).Msg("stat CID in IPFS")
			return nil, status.Errorf(codes.Internal, "stat CID in IPFS: %v", err)
		}

		if !exists {
			d.logger.Error().Str("cid", cid).Msg("CID not found in IPFS")
			return nil, status.Errorf(codes.NotFound, "CID not found in IPFS")
		}

		if pinning {
			if err := d.ipfsClient.PinCID(ctx, cid); err != nil {
				if d.cfg.PinFailurePolicy == pinFailurePolicyBestEffort {
					d.logger.Warn().Err(err).Str("cid", cid).Msg("pin CID in IPFS (best-effort mode)")
				} else {
					d.logger.Error().Err(err).Str("cid", cid).Msg("pin CID in IPFS")
					return nil, status.Errorf(codes.Internal, "pin CID in IPFS: %v", err)
				}
			}
		}

	case mfsPath != "":
		// Read-write: explicit MFS path provided.
		mfsPath, err = normalizeMFSPath(mfsPath, d.cfg.MFSRoot)
		if err != nil {
			d.logger.Error().Err(err).Str("mfsPath", mfsPath).Str("root", d.cfg.MFSRoot).Msg("invalid MFS path")
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		d.logger.Debug().Str("mfsPath", mfsPath).Msg("volume type: read-write, explicit MFS path")
		if err := d.ipfsClient.MkdirMFS(ctx, mfsPath); err != nil {
			d.logger.Error().Err(err).Str("mfsPath", mfsPath).Msg("make MFS directory in IPFS")
			return nil, status.Errorf(codes.Internal, "make MFS directory in IPFS: %v", err)
		}

	default:
		// Read-write: create a new path under MFS root (derived from volume name).
		mfsPath, err = normalizeMFSPath(d.cfg.MFSRoot+"/"+sanitizeVolumeID(name), d.cfg.MFSRoot)
		if err != nil {
			d.logger.Error().Err(err).Str("root", d.cfg.MFSRoot).Msg("invalid default MFS path")
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		d.logger.Debug().Str("mfsPath", mfsPath).Msg("volume type: read-write, auto MFS path")

		if err := d.ipfsClient.MkdirMFS(ctx, mfsPath); err != nil {
			d.logger.Error().Err(err).Str("mfsPath", mfsPath).Msg("make MFS directory in IPFS")
			return nil, status.Errorf(codes.Internal, "make MFS directory in IPFS: %v", err)
		}
	}

	if snapshotSourceID != "" {
		if err := d.restoreSnapshotToMFS(ctx, snapshotSourceID, mfsPath); err != nil {
			return nil, err
		}
	}

	volumeContext[paramCID] = cid
	volumeContext[paramMFSPath] = mfsPath
	volumeContext[paramReadOnly] = fmt.Sprintf("%t", readOnly)

	d.logger.Debug().
		Str("volumeID", volumeID).
		Str("mfsPath", mfsPath).
		Bool("readOnly", readOnly).
		Msg("create volume response prepared")

	volume := &csi.Volume{
		VolumeId:      volumeID,
		CapacityBytes: sizeBytes,
		VolumeContext: volumeContext,
	}
	if snapshotSourceID != "" {
		volume.ContentSource = &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Snapshot{
				Snapshot: &csi.VolumeContentSource_SnapshotSource{
					SnapshotId: snapshotSourceID,
				},
			},
		}
	}

	return &csi.CreateVolumeResponse{
		Volume: volume,
	}, nil
}

func snapshotSourceID(contentSource *csi.VolumeContentSource) (string, error) {
	if contentSource == nil {
		return "", nil
	}
	switch src := contentSource.GetType().(type) {
	case *csi.VolumeContentSource_Snapshot:
		if src.Snapshot == nil || src.Snapshot.SnapshotId == "" {
			return "", status.Error(codes.InvalidArgument, "snapshot content source requires snapshot ID")
		}
		return src.Snapshot.SnapshotId, nil
	case *csi.VolumeContentSource_Volume:
		return "", status.Error(codes.InvalidArgument, "volume content source is not supported")
	default:
		return "", status.Error(codes.InvalidArgument, "unsupported volume content source")
	}
}

func (d *Driver) restoreSnapshotToMFS(ctx context.Context, snapshotID, mfsPath string) error {
	exists, err := d.ipfsClient.StatCID(ctx, snapshotID)
	if err != nil {
		d.logger.Error().Err(err).Str("snapshot_id", snapshotID).Msg("stat snapshot CID in IPFS")
		return status.Errorf(codes.Internal, "stat snapshot CID in IPFS: %v", err)
	}

	if !exists {
		d.logger.Error().Str("snapshot_id", snapshotID).Msg("snapshot CID not found in IPFS")
		return status.Error(codes.NotFound, "snapshot CID not found in IPFS")
	}

	tmpDir, err := os.MkdirTemp("", "csi-ipfs-restore-*")
	if err != nil {
		d.logger.Error().Err(err).Msg("create temporary restore directory")
		return status.Errorf(codes.Internal, "create temporary restore directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			d.logger.Warn().Err(err).Msg("remove temporary restore directory")
		}
	}()

	if err := d.ipfsClient.GetCIDContent(ctx, snapshotID, tmpDir); err != nil {
		d.logger.Error().Err(err).Str("snapshot_id", snapshotID).Str(
			"temp_dir",
			tmpDir,
		).Msg("download snapshot CID content")
		return status.Errorf(codes.Internal, "download snapshot CID content: %v", err)
	}

	if err := d.ipfsClient.ImportToMFS(ctx, tmpDir, mfsPath); err != nil {
		d.logger.Error().Err(err).Str("snapshot_id", snapshotID).Str(
			"mfsPath",
			mfsPath,
		).Msg("restore snapshot content to MFS")
		return status.Errorf(codes.Internal, "restore snapshot content to MFS: %v", err)
	}

	return nil
}

// DeleteVolume implements csi.ControllerServer. It unpins/removes IPFS state using attributes from the PV.
//
// ctx bounds IPFS cleanup. req must set VolumeId.
// It returns an empty DeleteVolumeResponse when the PV is already gone (idempotent), or on success after cleanup.
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	d.logger.Info().Str("id", volumeID).Msg("delete volume")

	if acquired := d.volumeLocks.TryAcquire(volumeID); !acquired {
		d.logger.Debug().Str("id", volumeID).Msg("delete volume: operation already in progress")
		return nil, status.Errorf(codes.Aborted, "an operation with the given volume id %q already exists", volumeID)
	}
	defer d.volumeLocks.Release(volumeID)

	attrs, err := d.volumeStore.GetVolumeAttributes(ctx, volumeID)
	if err != nil {
		if errors.Is(err, store.ErrPVNotFound) {
			d.logger.Debug().Str("id", volumeID).Msg("PV not found, volume may already be deleted")
			return &csi.DeleteVolumeResponse{}, nil
		}

		d.logger.Error().Err(err).Str("id", volumeID).Msg("get volume attributes from PV")
		return nil, status.Errorf(codes.Internal, "get volume attributes: %v", err)
	}

	d.cleanupVolumeInIPFS(ctx, attrs)
	d.logger.Debug().Str("id", volumeID).Msg("delete volume completed")
	return &csi.DeleteVolumeResponse{}, nil
}

// ValidateVolumeCapabilities implements csi.ControllerServer and checks PV existence and access modes.
//
// ctx is used for PV lookup. req must set VolumeId and at least one VolumeCapability.
// It returns Confirmed capabilities when supported, a message-only response when an access mode is unsupported,
// or NotFound/Internal if the volume cannot be read.
func (d *Driver) ValidateVolumeCapabilities(
	ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest,
) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are required")
	}

	d.logger.Info().
		Str("id", volumeID).
		Int("capabilities", len(req.GetVolumeCapabilities())).
		Msg("validate volume capabilities")

	_, err := d.volumeStore.GetVolumeAttributes(ctx, volumeID)
	if err != nil {
		if errors.Is(err, store.ErrPVNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume %s not found", volumeID)
		}

		d.logger.Error().Err(err).Str("id", volumeID).Msg("get volume attributes")
		return nil, status.Errorf(codes.Internal, "get volume attributes: %v", err)
	}

	var confirmed *csi.ValidateVolumeCapabilitiesResponse_Confirmed
	for _, capability := range req.GetVolumeCapabilities() {
		if capability.GetAccessMode() != nil {
			mode := capability.GetAccessMode().GetMode()
			switch mode {
			case csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
				csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY:
				confirmed = &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
					VolumeCapabilities: req.GetVolumeCapabilities(),
				}
			default:
				return &csi.ValidateVolumeCapabilitiesResponse{
					Message: fmt.Sprintf("access mode %s not supported", mode),
				}, nil
			}
		}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{Confirmed: confirmed}, nil
}

// ControllerGetCapabilities implements csi.ControllerServer and lists supported controller RPCs.
//
// The request context and request value are unused.
// It always returns a populated ControllerGetCapabilitiesResponse for this driver build.
func (d *Driver) ControllerGetCapabilities(
	_ context.Context,
	_ *csi.ControllerGetCapabilitiesRequest,
) (*csi.ControllerGetCapabilitiesResponse, error) {
	d.logger.Debug().Msg("controller get capabilities")
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: buildControllerCapabilities(),
	}, nil
}

// ListVolumes implements csi.ControllerServer and lists volumes by listing PVs for this driver name.
//
// ctx is passed to the volume store. req may carry paging tokens (currently not applied to the lister).
// It returns ListVolumesResponse entries or Internal if listing fails.
func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	d.logger.Info().Msg("list volumes")

	volumes, err := d.volumeStore.ListVolumes(ctx, d.cfg.DriverName)
	if err != nil {
		d.logger.Error().Err(err).Msg("list volumes from Kubernetes")
		return nil, status.Errorf(codes.Internal, "list volumes: %v", err)
	}

	var entries []*csi.ListVolumesResponse_Entry
	for _, v := range volumes {
		entries = append(
			entries, &csi.ListVolumesResponse_Entry{
				Volume: &csi.Volume{
					VolumeId:      v.VolumeID,
					CapacityBytes: v.Capacity,
					VolumeContext: v.Attributes,
				},
			},
		)
	}

	sort.Slice(
		entries, func(i, j int) bool {
			return entries[i].GetVolume().GetVolumeId() < entries[j].GetVolume().GetVolumeId()
		},
	)

	start := 0
	if token := req.GetStartingToken(); token != "" {
		startIdx, err := strconv.Atoi(token)
		if err != nil || startIdx < 0 || startIdx > len(entries) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid starting token %q", token)
		}
		start = startIdx
	}

	maxEntries := int(req.GetMaxEntries())
	if maxEntries <= 0 {
		maxEntries = len(entries) - start
	}
	end := start + maxEntries
	if end > len(entries) {
		end = len(entries)
	}

	nextToken := ""
	if end < len(entries) {
		nextToken = strconv.Itoa(end)
	}

	paged := entries[start:end]
	d.logger.Debug().
		Int("count", len(paged)).
		Str("next_token", nextToken).
		Msg("list volumes completed")

	return &csi.ListVolumesResponse{
		Entries:   paged,
		NextToken: nextToken,
	}, nil
}

// GetCapacity implements csi.ControllerServer and returns free space derived from IPFS repo/stat.
//
// ctx bounds the IPFS call. req may include capability hints (ignored for this implementation).
// It returns AvailableCapacity in bytes or Internal if repo statistics cannot be read.
func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	d.logger.Debug().
		Int("capabilities", len(req.GetVolumeCapabilities())).
		Interface("params", req.GetParameters()).
		Msg("get capacity")

	repoStat, err := d.ipfsClient.GetRepoStat(ctx)
	if err != nil {
		d.logger.Error().Err(err).Msg("get repo stat in IPFS")
		return nil, status.Errorf(codes.Internal, "get repo stat in IPFS: %v", err)
	}

	var available uint64
	if repoStat.StorageMax <= repoStat.RepoSize {
		d.logger.Warn().
			Uint64("storageMax", repoStat.StorageMax).
			Uint64("repoSize", repoStat.RepoSize).
			Msg("repo usage exceeds configured storage max, clamping available capacity to zero")
	} else {
		available = repoStat.StorageMax - repoStat.RepoSize
	}
	d.logger.Debug().Uint64("available", available).Uint64("repoSize", repoStat.RepoSize).Msg("capacity calculated")

	return &csi.GetCapacityResponse{
		AvailableCapacity: int64(available),
	}, nil
}

// --- ControllerPublishVolume / ControllerUnpublishVolume: not used when attachRequired is false ---

// ControllerPublishVolume is unimplemented; this driver does not use attach-required workflows.
func (d *Driver) ControllerPublishVolume(
	context.Context, *csi.ControllerPublishVolumeRequest,
) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerPublishVolume not supported")
}

// ControllerUnpublishVolume is unimplemented; this driver does not use attach-required workflows.
func (d *Driver) ControllerUnpublishVolume(
	context.Context, *csi.ControllerUnpublishVolumeRequest,
) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "ControllerUnpublishVolume not supported")
}

// CreateSnapshot implements csi.ControllerServer. It resolves the source volume MFS path to a CID,
// pins that CID, and returns the CID as SnapshotId (read-only CID volumes cannot be snapshotted).
//
// ctx bounds IPFS and PV lookup. req must set Name and SourceVolumeId.
// It returns CreateSnapshotResponse with ReadyToUse snapshot metadata, or NotFound/InvalidArgument/Internal errors.
func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse, error,
) {
	name := req.GetName()
	sourceVolumeID := req.GetSourceVolumeId()

	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot name is required")
	}

	if sourceVolumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "source volume ID is required")
	}

	d.logger.Info().
		Str("name", name).
		Str("source_volume_id", sourceVolumeID).
		Msg("create snapshot")

	lockKey := "snapshot-" + name
	if acquired := d.volumeLocks.TryAcquire(lockKey); !acquired {
		return nil, status.Errorf(codes.Aborted, "an operation for snapshot %q already exists", name)
	}
	defer d.volumeLocks.Release(lockKey)

	attrs, err := d.volumeStore.GetVolumeAttributes(ctx, sourceVolumeID)
	if err != nil {
		if errors.Is(err, store.ErrPVNotFound) {
			return nil, status.Errorf(codes.NotFound, "source volume %s not found", sourceVolumeID)
		}

		return nil, status.Errorf(codes.Internal, "get source volume: %v", err)
	}

	mfsPath := attrs.Attributes[paramMFSPath]
	if mfsPath == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"source volume has no MFS path (read-only by CID volumes cannot be snapshotted)",
		)
	}

	cid, err := d.ipfsClient.ResolveMFSCID(ctx, mfsPath)
	if err != nil {
		d.logger.Error().Err(err).Str("mfsPath", mfsPath).Msg("resolve MFS CID for snapshot")
		return nil, status.Errorf(codes.Internal, "resolve MFS CID: %v", err)
	}

	if err := d.ipfsClient.PinCID(ctx, cid); err != nil {
		d.logger.Warn().Err(err).Str("cid", cid).Msg("pin snapshot CID")
	}

	sizeBytes := attrs.Capacity
	if sizeBytes == 0 {
		sizeBytes = 1 * 1024 * 1024 * 1024
	}

	d.logger.Debug().Str("snapshot_id", cid).Str("source_volume_id", sourceVolumeID).Msg("snapshot created")
	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     cid,
			SourceVolumeId: sourceVolumeID,
			CreationTime:   timestamppb.Now(),
			ReadyToUse:     true,
			SizeBytes:      sizeBytes,
		},
	}, nil
}

// DeleteSnapshot implements csi.ControllerServer and unpins the snapshot CID in the local repo.
//
// ctx bounds the IPFS unpin. req must set SnapshotId to the CID string.
// It returns an empty DeleteSnapshotResponse on success (warnings only if unpin fails).
func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse, error,
) {
	snapshotID := req.GetSnapshotId()
	if snapshotID == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot ID is required")
	}

	d.logger.Info().Str("snapshot_id", snapshotID).Msg("delete snapshot")

	lockKey := "delete-snapshot-" + snapshotID
	if acquired := d.volumeLocks.TryAcquire(lockKey); !acquired {
		return nil, status.Errorf(codes.Aborted, "an operation for snapshot %q already exists", snapshotID)
	}
	defer d.volumeLocks.Release(lockKey)

	volumeRefs, snapshotRefs, err := d.cidReferenceUsage(ctx, snapshotID, "", true)
	if err != nil {
		if errors.Is(err, errSnapshotStoreUnavailable) {
			return nil, status.Error(
				codes.FailedPrecondition,
				"snapshot reference checks are unavailable (snapshot store disabled)",
			)
		}

		d.logger.Error().Err(err).Str("snapshot_id", snapshotID).Msg("check snapshot CID references before unpin")
		return nil, status.Errorf(codes.Internal, "check snapshot references: %v", err)
	}
	if volumeRefs > 0 || snapshotRefs > 0 {
		d.logger.Info().
			Str("snapshot_id", snapshotID).
			Int("volume_refs", volumeRefs).
			Int("snapshot_refs", snapshotRefs).
			Msg("skip snapshot CID unpin: referenced by other resources")
		return &csi.DeleteSnapshotResponse{}, nil
	}

	if err := d.ipfsClient.UnpinCID(ctx, snapshotID); err != nil {
		d.logger.Warn().Err(err).Str("snapshot_id", snapshotID).Msg("unpin snapshot CID")
	}

	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots implements csi.ControllerServer and lists snapshots from VolumeSnapshotContent objects.
//
// ctx is passed to the snapshot store. req may filter by SnapshotId and SourceVolumeId and limit MaxEntries.
// It returns Entries or nil when the snapshot store is unavailable; Internal if listing fails.
func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	if d.snapshotStore == nil {
		d.logger.Debug().Msg("list snapshots: snapshot store not available (CRD not installed?)")
		return &csi.ListSnapshotsResponse{Entries: nil}, nil
	}

	entries, err := d.snapshotStore.ListSnapshots(ctx, d.cfg.DriverName, req.GetSnapshotId(), req.GetSourceVolumeId())
	if err != nil {
		d.logger.Error().Err(err).Msg("list snapshots from Kubernetes")
		return nil, status.Errorf(codes.Internal, "list snapshots: %v", err)
	}

	maxEntries := int(req.GetMaxEntries())
	if maxEntries <= 0 {
		maxEntries = len(entries)
	}

	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	csiEntries := make([]*csi.ListSnapshotsResponse_Entry, 0, len(entries))
	for _, e := range entries {
		csiEntries = append(csiEntries, snapshotEntryToCSI(e))
	}

	d.logger.Debug().Int("count", len(csiEntries)).Msg("list snapshots completed")
	return &csi.ListSnapshotsResponse{Entries: csiEntries}, nil
}

// ControllerExpandVolume implements csi.ControllerServer and records the requested larger capacity.
//
// ctx is unused beyond RPC cancellation. req must set VolumeId and a positive CapacityRange.
// It returns ControllerExpandVolumeResponse with CapacityBytes and NodeExpansionRequired=true
// (no block-device resize in the controller).
func (d *Driver) ControllerExpandVolume(
	_ context.Context, req *csi.ControllerExpandVolumeRequest,
) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	sizeBytes := parseCapacityFromRequest(req.GetCapacityRange(), 0)
	if sizeBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "capacity range is required and must be positive")
	}

	d.logger.Info().Str("volume_id", volumeID).Int64("size_bytes", sizeBytes).Msg("controller expand volume")
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         sizeBytes,
		NodeExpansionRequired: true,
	}, nil
}

// ControllerGetVolume implements csi.ControllerServer and returns current volume metadata from the PV.
//
// ctx is used for PV lookup. req must set VolumeId.
// It returns ControllerGetVolumeResponse with Volume details, or NotFound/Internal on error.
func (d *Driver) ControllerGetVolume(
	ctx context.Context, req *csi.ControllerGetVolumeRequest,
) (*csi.ControllerGetVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	attrs, err := d.volumeStore.GetVolumeAttributes(ctx, volumeID)
	if err != nil {
		if errors.Is(err, store.ErrPVNotFound) {
			return nil, status.Errorf(codes.NotFound, "volume %s not found", volumeID)
		}
		return nil, status.Errorf(codes.Internal, "get volume: %v", err)
	}

	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      attrs.VolumeID,
			CapacityBytes: attrs.Capacity,
			VolumeContext: attrs.Attributes,
		},
	}, nil
}

// ControllerModifyVolume implements csi.ControllerServer as a no-op (volume context is not mutable here).
//
// ctx is unused. req must set VolumeId.
// It returns an empty ControllerModifyVolumeResponse on success.
func (d *Driver) ControllerModifyVolume(
	ctx context.Context, req *csi.ControllerModifyVolumeRequest,
) (*csi.ControllerModifyVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	d.logger.Debug().Str("volume_id", volumeID).Msg("controller modify volume (no-op)")
	return &csi.ControllerModifyVolumeResponse{}, nil
}
