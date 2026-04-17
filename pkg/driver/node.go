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
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/mount-utils"
)

// NodeStageVolume implements csi.NodeServer: materializes IPFS content into the staging directory.
//
// ctx bounds fetch work. req must set VolumeId and StagingTargetPath; VolumeContext supplies cid/mfspath.
// It returns an empty NodeStageVolumeResponse on success, or a gRPC error if mkdir/fetch fails.
func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse, error,
) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	stagingPath := req.GetStagingTargetPath()
	if stagingPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}

	d.logger.Info().
		Str("id", volumeID).
		Str("stagingPath", stagingPath).
		Msg("node stage volume")

	if err := os.MkdirAll(stagingPath, 0750); err != nil {
		d.logger.Error().Err(err).Str("stagingPath", stagingPath).Msg("make staging directory")
		return nil, status.Errorf(codes.Internal, "make staging directory: %v", err)
	}

	if err := d.fetchContentToPath(ctx, req.GetVolumeContext(), stagingPath); err != nil {
		return nil, err
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume implements csi.NodeServer: for read-write MFS volumes, imports staging back to MFS, then unmounts.
//
// ctx bounds IPFS import. req must set VolumeId and StagingTargetPath; readonly/mfspath are read from the PV if needed.
// It returns an empty NodeUnstageVolumeResponse; import/cleanup failures are logged as warnings where appropriate.
func (d *Driver) NodeUnstageVolume(
	ctx context.Context, req *csi.NodeUnstageVolumeRequest,
) (*csi.NodeUnstageVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	stagingPath := req.GetStagingTargetPath()
	if stagingPath == "" {
		return nil, status.Error(codes.InvalidArgument, "staging target path is required")
	}

	d.logger.Info().
		Str("id", volumeID).
		Str("stagingPath", stagingPath).
		Msg("node unstage volume")

	// NodeUnstageVolumeRequest does not include VolumeContext; resolve attributes from the PV.
	attrs, err := d.volumeStore.GetVolumeAttributes(ctx, volumeID)
	var mfsPath string
	var readOnly bool

	if err == nil {
		mfsPath = attrs.Attributes[paramMFSPath]
		readOnly = attrs.Attributes[paramReadOnly] == "true"
	} else {
		d.logger.Debug().Err(err).Str(
			"id",
			volumeID,
		).Msg("could not get volume attributes from PV, skipping MFS import")
	}

	if !readOnly && mfsPath != "" {
		d.logger.Debug().Str("mfsPath", mfsPath).Msg("unstage: import staging dir back to MFS")
		if err := d.ipfsClient.ImportToMFS(ctx, stagingPath, mfsPath); err != nil {
			d.logger.Error().Err(err).Str("mfsPath", mfsPath).Str(
				"stagingPath",
				stagingPath,
			).Msg("import to MFS from IPFS")
			return nil, status.Errorf(codes.Internal, "import to MFS from IPFS: %v", err)
		}
	}

	mounter := mount.New("")
	if err := mount.CleanupMountPoint(stagingPath, mounter, true); err != nil {
		d.logger.Warn().Err(err).Str("stagingPath", stagingPath).Msg("cleanup staging path")
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodePublishVolume implements csi.NodeServer and bind-mounts the staged (or per-volume) path to TargetPath.
//
// ctx bounds setup work. req must set VolumeId and TargetPath; StagingTargetPath may be empty to use MountDir layout.
// It returns an empty NodePublishVolumeResponse when the bind mount succeeds or is already present (idempotent).
func (d *Driver) NodePublishVolume(
	ctx context.Context, req *csi.NodePublishVolumeRequest,
) (*csi.NodePublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}

	stagingPath := req.GetStagingTargetPath()
	readOnly := req.GetReadonly()
	volumeContext := req.GetVolumeContext()

	d.logger.Info().
		Str("id", volumeID).
		Str("targetPath", targetPath).
		Str("stagingPath", stagingPath).
		Bool("readOnly", readOnly).
		Msg("node publish volume")

	sourcePath := stagingPath
	if sourcePath == "" {
		sourcePath = filepath.Join(d.cfg.MountDir, volumeID)

		d.logger.Debug().Str("sourcePath", sourcePath).Msg("no staging path, using mount dir")
		if err := os.MkdirAll(sourcePath, 0750); err != nil {
			d.logger.Error().Err(err).Str("sourcePath", sourcePath).Msg("make source directory")
			return nil, status.Errorf(codes.Internal, "make source directory: %v", err)
		}

		if err := d.fetchContentToPath(ctx, volumeContext, sourcePath); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(targetPath, 0750); err != nil {
		d.logger.Error().Err(err).Str("targetPath", targetPath).Msg("make target directory")
		return nil, status.Errorf(codes.Internal, "make target directory: %v", err)
	}

	mounter := mount.New("")
	notMnt, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Errorf(codes.Internal, "make target directory: %v", err)
			}
			notMnt = true
		} else {
			d.logger.Error().Err(err).Str("targetPath", targetPath).Msg("check mount point")
			return nil, status.Errorf(codes.Internal, "check mount point: %v", err)
		}
	}

	if !notMnt {
		d.logger.Debug().Str("targetPath", targetPath).Msg("already mounted, idempotent success")
		return &csi.NodePublishVolumeResponse{}, nil
	}

	options := buildMountOptions(req)
	if err := mounter.Mount(sourcePath, targetPath, "", options); err != nil {
		d.logger.Error().Err(err).
			Str("sourcePath", sourcePath).
			Str("targetPath", targetPath).
			Msg("bind mount failed")
		return nil, status.Errorf(codes.Internal, "mount: %v", err)
	}

	d.logger.Debug().Str("targetPath", targetPath).Msg("node publish volume completed")
	return &csi.NodePublishVolumeResponse{}, nil
}

func buildMountOptions(req *csi.NodePublishVolumeRequest) []string {
	options := []string{"bind"}
	seen := map[string]struct{}{
		"bind": {},
	}

	if req.GetReadonly() {
		options = append(options, "ro")
		seen["ro"] = struct{}{}
	}

	mount := req.GetVolumeCapability().GetMount()
	if mount == nil {
		return options
	}
	for _, flag := range mount.GetMountFlags() {
		if flag == "" {
			continue
		}
		if _, exists := seen[flag]; exists {
			continue
		}
		seen[flag] = struct{}{}
		options = append(options, flag)
	}

	return options
}

// NodeUnpublishVolume implements csi.NodeServer and unmounts the published target path.
//
// ctx is unused beyond cancellation. req must set VolumeId and TargetPath.
// It returns an empty NodeUnpublishVolumeResponse on success or Internal if cleanup fails fatally.
func (d *Driver) NodeUnpublishVolume(
	ctx context.Context, req *csi.NodeUnpublishVolumeRequest,
) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	targetPath := req.GetTargetPath()
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target path is required")
	}

	d.logger.Info().
		Str("id", volumeID).
		Str("targetPath", targetPath).
		Msg("node unpublish volume")

	mounter := mount.New("")
	if err := mount.CleanupMountPoint(targetPath, mounter, true); err != nil {
		d.logger.Error().Err(err).Str("targetPath", targetPath).Msg("cleanup target path")
		return nil, status.Errorf(codes.Internal, "cleanup target path: %v", err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetInfo implements csi.NodeServer and returns the configured NodeId.
//
// ctx is unused. The request is ignored.
// It returns NodeGetInfoResponse with NodeId set from Config.
func (d *Driver) NodeGetInfo(ctx context.Context, _ *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	d.logger.Debug().Str("nodeID", d.cfg.NodeID).Msg("node get info")
	return &csi.NodeGetInfoResponse{NodeId: d.cfg.NodeID}, nil
}

// NodeGetCapabilities implements csi.NodeServer and advertises stage/unstage, stats, and expand support.
//
// Context and request are unused.
// It returns NodeGetCapabilitiesResponse with the driver's RPC capabilities.
func (d *Driver) NodeGetCapabilities(
	_ context.Context, _ *csi.NodeGetCapabilitiesRequest,
) (*csi.NodeGetCapabilitiesResponse, error) {
	d.logger.Debug().Msg("node get capabilities")
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: buildNodeCapabilities(),
	}, nil
}

// NodeGetVolumeStats implements csi.NodeServer and reports byte and inode usage via statfs on VolumePath.
//
// ctx is unused beyond RPC lifetime. req must set VolumeId and VolumePath.
// It returns NodeGetVolumeStatsResponse with BYTES and INODES usage entries, or Internal if statfs fails.
func (d *Driver) NodeGetVolumeStats(
	ctx context.Context, req *csi.NodeGetVolumeStatsRequest,
) (*csi.NodeGetVolumeStatsResponse, error) {
	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID is required")
	}

	volumePath := req.GetVolumePath()
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is required")
	}

	d.logger.Debug().
		Str("id", volumeID).
		Str("path", volumePath).
		Msg("node get volume stats")

	stats, err := getVolumeStats(volumePath)
	if err != nil {
		d.logger.Error().Err(err).Str("path", volumePath).Msg("get volume stats")
		return nil, status.Errorf(codes.Internal, "get volume stats: %v", err)
	}

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: stats.availableBytes,
				Total:     stats.totalBytes,
				Used:      stats.usedBytes,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Available: stats.availableInodes,
				Total:     stats.totalInodes,
				Used:      stats.usedInodes,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}, nil
}

// NodeExpandVolume implements csi.NodeServer and acknowledges expansion for bind-mounted content (no block resize).
//
// ctx is unused. req must set VolumePath; CapacityRange selects the reported size (defaults applied when missing).
// It returns NodeExpandVolumeResponse with CapacityBytes set to the acknowledged size.
func (d *Driver) NodeExpandVolume(
	ctx context.Context, req *csi.NodeExpandVolumeRequest,
) (*csi.NodeExpandVolumeResponse, error) {
	volumePath := req.GetVolumePath()
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "volume path is required")
	}

	sizeBytes := parseCapacityFromRequest(req.GetCapacityRange(), defaultCapacityBytes)

	d.logger.Debug().
		Str("volume_path", volumePath).
		Int64("size_bytes", sizeBytes).
		Msg("node expand volume (no-op for bind mount)")
	return &csi.NodeExpandVolumeResponse{
		CapacityBytes: sizeBytes,
	}, nil
}

// fetchContentToPath downloads or exports IPFS content into destPath using volumeContext keys cid/mfspath.
//
// ctx bounds IPFS I/O. volumeContext may be nil or omit both keys, in which case the function is a no-op.
// It returns nil on success, or a gRPC-wrapped error when CID export fails critically.
func (d *Driver) fetchContentToPath(ctx context.Context, volumeContext map[string]string, destPath string) error {
	if volumeContext == nil {
		return nil
	}

	cid := volumeContext[paramCID]
	mfsPath := volumeContext[paramMFSPath]

	switch {
	case cid != "":
		if err := ensureEmptyDirectory(destPath); err != nil {
			d.logger.Error().Err(err).Str("destPath", destPath).Msg("prepare destination directory")
			return status.Errorf(codes.Internal, "prepare destination directory: %v", err)
		}

		d.logger.Debug().Str("cid", cid).Str("destPath", destPath).Msg("fetch content by CID")
		if err := d.ipfsClient.GetCIDContent(ctx, cid, destPath); err != nil {
			d.logger.Error().Err(err).Str("cid", cid).Str("destPath", destPath).Msg("get CID content from IPFS")
			return status.Errorf(codes.Internal, "get CID content from IPFS: %v", err)
		}
	case mfsPath != "":
		if err := ensureEmptyDirectory(destPath); err != nil {
			d.logger.Error().Err(err).Str("destPath", destPath).Msg("prepare destination directory")
			return status.Errorf(codes.Internal, "prepare destination directory: %v", err)
		}

		d.logger.Debug().Str("mfsPath", mfsPath).Str("destPath", destPath).Msg("export from MFS")
		if err := d.ipfsClient.ExportMFS(ctx, mfsPath, destPath); err != nil {
			d.logger.Error().Err(err).Str("mfsPath", mfsPath).Str("destPath", destPath).Msg("export MFS from IPFS")
			return status.Errorf(codes.Internal, "export MFS from IPFS: %v", err)
		}
	default:
		d.logger.Debug().Msg("no cid/mfspath in context, skipping content fetch")
	}
	return nil
}

func ensureEmptyDirectory(path string) error {
	if err := os.MkdirAll(path, 0750); err != nil {
		return fmt.Errorf("create directory %s: %w", path, err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", path, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("remove existing entry %s: %w", entryPath, err)
		}
	}

	return nil
}

// buildNodeCapabilities returns the list of node RPC capabilities supported by this driver.
func buildNodeCapabilities() []*csi.NodeServiceCapability {
	types := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	}
	caps := make([]*csi.NodeServiceCapability, 0, len(types))
	for _, t := range types {
		caps = append(
			caps, &csi.NodeServiceCapability{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{Type: t},
				},
			},
		)
	}
	return caps
}
