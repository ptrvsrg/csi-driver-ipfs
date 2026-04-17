// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an 'AS IS' BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store"
	storemocks "github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store/mocks"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/ipfs"
	ipfsmocks "github.com/ptrvsrg/csi-driver-ipfs/pkg/ipfs/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCreateVolume_ReadOnlyCID(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)
	snapshotStore := storemocks.NewMockSnapshotStore(t)

	ipfsClient.On("StatCID", mock.Anything, "cid-a").Return(true, nil).Once()
	ipfsClient.On("PinCID", mock.Anything, "cid-a").Return(nil).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
			MFSRoot:    "/csi-volumes",
		},
		ipfsClient,
		volumeStore,
		snapshotStore,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	resp, err := d.CreateVolume(
		context.Background(), &csi.CreateVolumeRequest{
			Name: "vol-a",
			Parameters: map[string]string{
				paramCID:     "cid-a",
				paramPinning: "true",
			},
		},
	)
	if err != nil {
		t.Fatalf("CreateVolume() error = %v", err)
	}

	if resp.GetVolume().GetVolumeContext()[paramCID] != "cid-a" {
		t.Fatalf("CreateVolume() cid context mismatch")
	}
	if resp.GetVolume().GetVolumeContext()[paramReadOnly] != "true" {
		t.Fatalf("CreateVolume() readonly context mismatch")
	}
}

func TestCreateVolume_PinFailureStrictMode(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On("StatCID", mock.Anything, "cid-a").Return(true, nil).Once()
	ipfsClient.On("PinCID", mock.Anything, "cid-a").Return(errors.New("pin failed")).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName:       DefaultDriverName,
			NodeID:           "node-a",
			MFSRoot:          "/csi-volumes",
			PinFailurePolicy: pinFailurePolicyStrict,
		},
		ipfsClient,
		storemocks.NewMockVolumeStore(t),
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.CreateVolume(
		context.Background(), &csi.CreateVolumeRequest{
			Name: "vol-a",
			Parameters: map[string]string{
				paramCID:     "cid-a",
				paramPinning: "true",
			},
		},
	)
	if status.Code(err) != codes.Internal {
		t.Fatalf("CreateVolume() code = %v, want %v", status.Code(err), codes.Internal)
	}
}

func TestCreateVolume_PinFailureBestEffortMode(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On("StatCID", mock.Anything, "cid-a").Return(true, nil).Once()
	ipfsClient.On("PinCID", mock.Anything, "cid-a").Return(errors.New("pin failed")).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName:       DefaultDriverName,
			NodeID:           "node-a",
			MFSRoot:          "/csi-volumes",
			PinFailurePolicy: pinFailurePolicyBestEffort,
		},
		ipfsClient,
		storemocks.NewMockVolumeStore(t),
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.CreateVolume(
		context.Background(), &csi.CreateVolumeRequest{
			Name: "vol-a",
			Parameters: map[string]string{
				paramCID:     "cid-a",
				paramPinning: "true",
			},
		},
	)
	if err != nil {
		t.Fatalf("CreateVolume() unexpected error = %v", err)
	}
}

func TestCreateVolume_InvalidMFSPath(t *testing.T) {
	t.Parallel()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
			MFSRoot:    "/csi-volumes",
		},
		ipfsmocks.NewMockClient(t),
		storemocks.NewMockVolumeStore(t),
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.CreateVolume(
		context.Background(), &csi.CreateVolumeRequest{
			Name: "vol-a",
			Parameters: map[string]string{
				paramMFSPath: "/csi-volumes/../../etc",
			},
		},
	)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateVolume() code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestCreateVolume_FromSnapshotSeedsMFS(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On("MkdirMFS", mock.Anything, "/csi-volumes/restore-vol").Return(nil).Once()
	ipfsClient.On("StatCID", mock.Anything, "snap-cid").Return(true, nil).Once()
	ipfsClient.On("GetCIDContent", mock.Anything, "snap-cid", mock.Anything).Return(nil).Once()
	ipfsClient.On("ImportToMFS", mock.Anything, mock.Anything, "/csi-volumes/restore-vol").Return(nil).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
			MFSRoot:    "/csi-volumes",
		},
		ipfsClient,
		storemocks.NewMockVolumeStore(t),
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	resp, err := d.CreateVolume(
		context.Background(), &csi.CreateVolumeRequest{
			Name: "restore-vol",
			VolumeContentSource: &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: "snap-cid"},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("CreateVolume() unexpected error = %v", err)
	}
	if got := resp.GetVolume().GetVolumeContext()[paramMFSPath]; got != "/csi-volumes/restore-vol" {
		t.Fatalf("CreateVolume() mfsPath = %q, want %q", got, "/csi-volumes/restore-vol")
	}
	if got := resp.GetVolume().GetVolumeContext()[paramReadOnly]; got != "false" {
		t.Fatalf("CreateVolume() readonly context = %q, want false", got)
	}
	if resp.GetVolume().GetContentSource() == nil {
		t.Fatalf("CreateVolume() content source = nil, want snapshot source")
	}
	gotSnapshot := resp.GetVolume().GetContentSource().GetSnapshot()
	if gotSnapshot == nil || gotSnapshot.GetSnapshotId() != "snap-cid" {
		t.Fatalf("CreateVolume() content source snapshot = %v, want snap-cid", gotSnapshot)
	}
}

func TestDeleteVolume_SharedCIDDoesNotUnpin(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)
	snapshotStore := storemocks.NewMockSnapshotStore(t)

	volumeStore.On("GetVolumeAttributes", mock.Anything, "vol-a").Return(
		&store.VolumeAttributes{
			VolumeID: "vol-a",
			Attributes: map[string]string{
				paramCID:      "cid-shared",
				paramPinning:  "true",
				paramReadOnly: "true",
			},
		}, nil,
	).Once()
	volumeStore.On("ListVolumes", mock.Anything, DefaultDriverName).Return(
		[]*store.VolumeAttributes{
			{
				VolumeID: "vol-a",
				Attributes: map[string]string{
					paramCID:     "cid-shared",
					paramPinning: "true",
				},
			},
			{
				VolumeID: "vol-b",
				Attributes: map[string]string{
					paramCID:     "cid-shared",
					paramPinning: "true",
				},
			},
		}, nil,
	).Once()
	snapshotStore.On(
		"ListSnapshots",
		mock.Anything,
		DefaultDriverName,
		"cid-shared",
		"",
	).Return([]store.SnapshotEntry{}, nil).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
		},
		ipfsClient,
		volumeStore,
		snapshotStore,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{VolumeId: "vol-a"})
	if err != nil {
		t.Fatalf("DeleteVolume() error = %v", err)
	}
	ipfsClient.AssertNotCalled(t, "UnpinCID", mock.Anything, "cid-shared")
}

func TestDeleteSnapshot_LastOwnerUnpins(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)
	snapshotStore := storemocks.NewMockSnapshotStore(t)

	volumeStore.On(
		"ListVolumes",
		mock.Anything,
		DefaultDriverName,
	).Return([]*store.VolumeAttributes{}, nil).Once()
	snapshotStore.On("ListSnapshots", mock.Anything, DefaultDriverName, "cid-last", "").Return(
		[]store.SnapshotEntry{
			{SnapshotID: "cid-last"},
		}, nil,
	).Once()
	ipfsClient.On("UnpinCID", mock.Anything, "cid-last").Return(nil).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
		},
		ipfsClient,
		volumeStore,
		snapshotStore,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{SnapshotId: "cid-last"})
	if err != nil {
		t.Fatalf("DeleteSnapshot() error = %v", err)
	}
}

func TestDeleteSnapshot_FailsWhenSnapshotStoreUnavailable(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)
	volumeStore.On(
		"ListVolumes",
		mock.Anything,
		DefaultDriverName,
	).Return([]*store.VolumeAttributes{}, nil).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
		},
		ipfsClient,
		volumeStore,
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.DeleteSnapshot(context.Background(), &csi.DeleteSnapshotRequest{SnapshotId: "cid-last"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("DeleteSnapshot() code = %v, want %v", status.Code(err), codes.FailedPrecondition)
	}
	ipfsClient.AssertNotCalled(t, "UnpinCID", mock.Anything, mock.Anything)
}

func TestListVolumes_Pagination(t *testing.T) {
	t.Parallel()

	volumeStore := storemocks.NewMockVolumeStore(t)
	volumeStore.On("ListVolumes", mock.Anything, DefaultDriverName).Return(
		[]*store.VolumeAttributes{
			{VolumeID: "vol-b", Capacity: 1, Attributes: map[string]string{}},
			{VolumeID: "vol-a", Capacity: 2, Attributes: map[string]string{}},
			{VolumeID: "vol-c", Capacity: 3, Attributes: map[string]string{}},
		}, nil,
	).Twice()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{DriverName: DefaultDriverName, NodeID: "node-a"},
		ipfsmocks.NewMockClient(t),
		volumeStore,
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	resp, err := d.ListVolumes(context.Background(), &csi.ListVolumesRequest{MaxEntries: 2})
	if err != nil {
		t.Fatalf("ListVolumes() unexpected error = %v", err)
	}
	if len(resp.GetEntries()) != 2 {
		t.Fatalf("ListVolumes() len = %d, want 2", len(resp.GetEntries()))
	}
	if got := resp.GetEntries()[0].GetVolume().GetVolumeId(); got != "vol-a" {
		t.Fatalf("ListVolumes() first id = %q, want vol-a", got)
	}
	if resp.GetNextToken() != "2" {
		t.Fatalf("ListVolumes() nextToken = %q, want 2", resp.GetNextToken())
	}

	resp, err = d.ListVolumes(
		context.Background(), &csi.ListVolumesRequest{
			StartingToken: resp.GetNextToken(),
			MaxEntries:    2,
		},
	)
	if err != nil {
		t.Fatalf("ListVolumes(page2) unexpected error = %v", err)
	}
	if len(resp.GetEntries()) != 1 || resp.GetEntries()[0].GetVolume().GetVolumeId() != "vol-c" {
		t.Fatalf("ListVolumes(page2) entries mismatch")
	}
	if resp.GetNextToken() != "" {
		t.Fatalf("ListVolumes(page2) nextToken = %q, want empty", resp.GetNextToken())
	}
}

func TestListVolumes_InvalidStartingToken(t *testing.T) {
	t.Parallel()

	volumeStore := storemocks.NewMockVolumeStore(t)
	volumeStore.On(
		"ListVolumes",
		mock.Anything,
		DefaultDriverName,
	).Return([]*store.VolumeAttributes{}, nil).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{DriverName: DefaultDriverName, NodeID: "node-a"},
		ipfsmocks.NewMockClient(t),
		volumeStore,
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.ListVolumes(context.Background(), &csi.ListVolumesRequest{StartingToken: "bad-token"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("ListVolumes() code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestValidateVolumeCapabilities_PVNotFound(t *testing.T) {
	t.Parallel()

	volumeStore := storemocks.NewMockVolumeStore(t)
	volumeStore.On("GetVolumeAttributes", mock.Anything, "vol-x").Return(
		(*store.VolumeAttributes)(nil),
		store.ErrPVNotFound,
	).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
		},
		ipfsmocks.NewMockClient(t),
		volumeStore,
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.ValidateVolumeCapabilities(
		context.Background(), &csi.ValidateVolumeCapabilitiesRequest{
			VolumeId: "vol-x",
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
			},
		},
	)
	if status.Code(err) != codes.NotFound {
		t.Fatalf("ValidateVolumeCapabilities() code = %v, want %v", status.Code(err), codes.NotFound)
	}
}

func TestCreateVolume_StatCIDInternalError(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On(
		"StatCID",
		mock.Anything,
		"cid-err",
	).Return(false, errors.New("rpc failed")).Once()

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
			MFSRoot:    "/csi-volumes",
		},
		ipfsClient,
		storemocks.NewMockVolumeStore(t),
		nil,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	_, err = d.CreateVolume(
		context.Background(), &csi.CreateVolumeRequest{
			Name: "vol-a",
			Parameters: map[string]string{
				paramCID: "cid-err",
			},
		},
	)
	if status.Code(err) != codes.Internal {
		t.Fatalf("CreateVolume() code = %v, want %v", status.Code(err), codes.Internal)
	}
}

func TestGetCapacity_ClampsNegativeAvailableToZero(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On("GetRepoStat", mock.Anything).Return(
		&ipfs.RepoStatResult{
			RepoSize:   200,
			StorageMax: 100,
		}, nil,
	).Once()

	d := &Driver{
		logger:     zerolog.Nop(),
		ipfsClient: ipfsClient,
	}

	resp, err := d.GetCapacity(context.Background(), &csi.GetCapacityRequest{})
	if err != nil {
		t.Fatalf("GetCapacity() unexpected error = %v", err)
	}
	if resp.GetAvailableCapacity() != 0 {
		t.Fatalf("GetCapacity() available = %d, want 0", resp.GetAvailableCapacity())
	}
}

func TestNormalizeMFSPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawPath string
		root    string
		want    string
		wantErr bool
	}{
		{
			name:    "normalizes path under root",
			rawPath: "/csi-volumes/ns/../vol-a",
			root:    "/csi-volumes",
			want:    "/csi-volumes/vol-a",
		},
		{
			name:    "rejects traversal outside root",
			rawPath: "/csi-volumes/../../etc",
			root:    "/csi-volumes",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()

				got, err := normalizeMFSPath(tt.rawPath, tt.root)
				if tt.wantErr {
					if err == nil {
						t.Fatalf("normalizeMFSPath() error = nil, want error")
					}
					return
				}
				if err != nil {
					t.Fatalf("normalizeMFSPath() unexpected error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("normalizeMFSPath() = %q, want %q", got, tt.want)
				}
			},
		)
	}
}

func TestNewDriver_InvalidPinFailurePolicy(t *testing.T) {
	t.Parallel()

	_, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName:       DefaultDriverName,
			NodeID:           "node-a",
			PinFailurePolicy: "invalid",
		},
		ipfsmocks.NewMockClient(t),
		storemocks.NewMockVolumeStore(t),
		nil,
	)
	if !errors.Is(err, ErrInvalidPinFailurePolicy) {
		t.Fatalf("NewDriver() error = %v, want %v", err, ErrInvalidPinFailurePolicy)
	}
}
