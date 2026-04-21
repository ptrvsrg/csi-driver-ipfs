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
	"math"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9\-]`)

var errSnapshotStoreUnavailable = errors.New("snapshot store unavailable for snapshot reference check")

const defaultCapacityBytes = 1 * 1024 * 1024 * 1024 // 1 GiB

// parseCapacityFromRequest returns required or limit bytes from the capacity range, or defaultBytes if nil/zero.
func parseCapacityFromRequest(capRange *csi.CapacityRange, defaultBytes int64) int64 {
	if capRange == nil {
		return defaultBytes
	}
	size := capRange.GetRequiredBytes()
	if size == 0 {
		size = capRange.GetLimitBytes()
	}
	if size <= 0 {
		return defaultBytes
	}
	return size
}

func isPinningEnabled(attrs map[string]string) bool {
	return attrs[paramPinning] != "false"
}

// controllerCapabilityTypes returns the list of controller RPC capabilities supported by this driver.
func controllerCapabilityTypes() []csi.ControllerServiceCapability_RPC_Type {
	return []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	}
}

// buildControllerCapabilities returns CSI controller capability descriptors.
func buildControllerCapabilities() []*csi.ControllerServiceCapability {
	types := controllerCapabilityTypes()
	caps := make([]*csi.ControllerServiceCapability, 0, len(types))
	for _, t := range types {
		caps = append(
			caps, &csi.ControllerServiceCapability{
				Type: &csi.ControllerServiceCapability_Rpc{
					Rpc: &csi.ControllerServiceCapability_RPC{Type: t},
				},
			},
		)
	}
	return caps
}

// snapshotEntryToCSI converts a SnapshotEntry to a CSI ListSnapshots response entry.
func snapshotEntryToCSI(e store.SnapshotEntry) *csi.ListSnapshotsResponse_Entry {
	var creationTime *timestamppb.Timestamp
	if e.CreationTime > 0 {
		creationTime = timestamppb.New(time.Unix(e.CreationTime, 0))
	} else {
		creationTime = timestamppb.New(time.Time{})
	}
	return &csi.ListSnapshotsResponse_Entry{
		Snapshot: &csi.Snapshot{
			SnapshotId:     e.SnapshotID,
			SourceVolumeId: e.SourceVolumeID,
			CreationTime:   creationTime,
			ReadyToUse:     e.ReadyToUse,
			SizeBytes:      e.SizeBytes,
		},
	}
}

// cleanupVolumeInIPFS unpins the volume CID (when pinning was used) and removes the MFS tree for read-write volumes.
//
// ctx bounds IPFS calls. attrs must contain driver volume attribute keys (cid, mfspath, readonly, pinning).
// It does not return errors; failures are logged at warn level.
func (d *Driver) cleanupVolumeInIPFS(ctx context.Context, attrs *store.VolumeAttributes) {
	cid := attrs.Attributes[paramCID]
	mfsPath := attrs.Attributes[paramMFSPath]
	readOnly := attrs.Attributes[paramReadOnly] == "true"
	pinning := isPinningEnabled(attrs.Attributes)

	if cid != "" && pinning {
		volumeRefs, snapshotRefs, err := d.cidReferenceUsage(ctx, cid, attrs.VolumeID, false)
		if err != nil {
			d.logger.Warn().Err(err).Str("cid", cid).Msg("check CID references before unpin")
		} else if volumeRefs > 0 || snapshotRefs > 0 {
			d.logger.Info().
				Str("cid", cid).
				Int("volume_refs", volumeRefs).
				Int("snapshot_refs", snapshotRefs).
				Msg("skip CID unpin: referenced by other resources")
		} else {
			if err := d.ipfsClient.UnpinCID(ctx, cid); err != nil {
				d.logger.Warn().Err(err).Str("cid", cid).Msg("unpin CID in IPFS")
			}
		}
	}
	if mfsPath != "" && !readOnly {
		if err := d.ipfsClient.RmMFS(ctx, mfsPath); err != nil {
			d.logger.Warn().Err(err).Str("mfsPath", mfsPath).Msg("remove MFS path in IPFS")
		}
	}
}

func (d *Driver) cidReferenceUsage(
	ctx context.Context,
	cid string,
	excludedVolumeID string,
	excludeOneSnapshot bool,
) (int, int, error) {
	volumes, err := d.volumeStore.ListVolumes(ctx, d.cfg.DriverName)
	if err != nil {
		return 0, 0, fmt.Errorf("list volumes for CID %s: %w", cid, err)
	}

	volumeRefs := 0
	for _, v := range volumes {
		if v == nil || v.VolumeID == excludedVolumeID {
			continue
		}
		if v.Attributes[paramCID] != cid || !isPinningEnabled(v.Attributes) {
			continue
		}
		volumeRefs++
	}

	if d.snapshotStore == nil {
		if excludeOneSnapshot {
			return 0, 0, errSnapshotStoreUnavailable
		}
		return volumeRefs, 0, nil
	}

	snapshots, err := d.snapshotStore.ListSnapshots(ctx, d.cfg.DriverName, cid, "")
	if err != nil {
		return 0, 0, fmt.Errorf("list snapshots for CID %s: %w", cid, err)
	}

	snapshotRefs := len(snapshots)
	if excludeOneSnapshot && snapshotRefs > 0 {
		snapshotRefs--
	}

	return volumeRefs, snapshotRefs, nil
}

// --- Volume ID and filesystem helpers (used by controller and node) ---

// sanitizeVolumeID normalizes a volume name into a safe ID segment (lowercase, alphanumeric and hyphen, max 128 chars).
func sanitizeVolumeID(name string) string {
	s := strings.ToLower(name)
	s = sanitizeRe.ReplaceAllString(s, "-")
	if len(s) > 128 {
		s = s[:128]
	}
	return s
}

// normalizeMFSPath canonicalizes an MFS path and verifies it stays under root.
func normalizeMFSPath(rawPath string, root string) (string, error) {
	normalizedRoot := path.Clean(root)
	if !path.IsAbs(normalizedRoot) {
		normalizedRoot = "/" + normalizedRoot
	}

	normalizedPath := path.Clean(rawPath)
	if !path.IsAbs(normalizedPath) {
		normalizedPath = "/" + normalizedPath
	}

	if normalizedPath != normalizedRoot && !strings.HasPrefix(normalizedPath, normalizedRoot+"/") {
		return "", fmt.Errorf("MFS path must be under %s", normalizedRoot)
	}

	return normalizedPath, nil
}

// volumeStats holds filesystem usage for a path (used by NodeGetVolumeStats).
type volumeStats struct {
	availableBytes  int64
	totalBytes      int64
	usedBytes       int64
	availableInodes int64
	totalInodes     int64
	usedInodes      int64
}

// getVolumeStats returns block and inode usage for the given path via statfs.
func getVolumeStats(path string) (*volumeStats, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, fmt.Errorf("statfs on path %s: %w", path, err)
	}

	blockSize := uint64(stat.Bsize)
	usedBlocks := subtractUint64Saturating(stat.Blocks, stat.Bfree)

	return &volumeStats{
		availableBytes:  multiplyUint64ToInt64(stat.Bavail, blockSize),
		totalBytes:      multiplyUint64ToInt64(stat.Blocks, blockSize),
		usedBytes:       multiplyUint64ToInt64(usedBlocks, blockSize),
		availableInodes: uint64ToInt64Saturating(stat.Ffree),
		totalInodes:     uint64ToInt64Saturating(stat.Files),
		usedInodes:      uint64ToInt64Saturating(subtractUint64Saturating(stat.Files, stat.Ffree)),
	}, nil
}

func subtractUint64Saturating(a, b uint64) uint64 {
	if b >= a {
		return 0
	}
	return a - b
}

func uint64ToInt64Saturating(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	// #nosec G115 -- bounded above by math.MaxInt64 just above
	return int64(v)
}

func multiplyUint64ToInt64(a, b uint64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	if a > math.MaxInt64/b {
		return math.MaxInt64
	}
	// #nosec G115 -- product is bounded above by math.MaxInt64 just above
	return int64(a * b)
}
