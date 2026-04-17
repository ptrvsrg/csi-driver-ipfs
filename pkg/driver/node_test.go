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
	"os"
	"path/filepath"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store"
	storemocks "github.com/ptrvsrg/csi-driver-ipfs/pkg/driver/store/mocks"
	ipfsmocks "github.com/ptrvsrg/csi-driver-ipfs/pkg/ipfs/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFetchContentToPath_ReturnsErrorOnMFSExportFailure(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On(
		"ExportMFS",
		mock.Anything,
		"/csi-volumes/test",
		mock.Anything,
	).Return(errors.New("export failed")).Once()

	d := &Driver{
		logger:     zerolog.Nop(),
		ipfsClient: ipfsClient,
	}

	err := d.fetchContentToPath(
		context.Background(),
		map[string]string{paramMFSPath: "/csi-volumes/test"},
		t.TempDir(),
	)
	if status.Code(err) != codes.Internal {
		t.Fatalf("fetchContentToPath() code = %v, want %v", status.Code(err), codes.Internal)
	}
}

func TestFetchContentToPath_ClearsDestinationBeforeExport(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	oldFile := filepath.Join(dest, "pod1.txt")
	if err := os.WriteFile(oldFile, []byte("stale"), 0600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	ipfsClient := ipfsmocks.NewMockClient(t)
	ipfsClient.On("ExportMFS", mock.Anything, "/csi-volumes/test", dest).
		Run(
			func(_ mock.Arguments) {
				if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
					t.Fatalf("expected stale file to be removed before export, stat err: %v", err)
				}
			},
		).
		Return(nil).
		Once()

	d := &Driver{
		logger:     zerolog.Nop(),
		ipfsClient: ipfsClient,
	}

	if err := d.fetchContentToPath(
		context.Background(),
		map[string]string{paramMFSPath: "/csi-volumes/test"},
		dest,
	); err != nil {
		t.Fatalf("fetchContentToPath() unexpected error = %v", err)
	}
}

func TestNodeUnstageVolume_ReturnsErrorOnImportFailure(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)

	volumeStore.On("GetVolumeAttributes", mock.Anything, "vol-a").Return(
		&store.VolumeAttributes{
			VolumeID: "vol-a",
			Attributes: map[string]string{
				paramMFSPath:  "/csi-volumes/test",
				paramReadOnly: "false",
			},
		}, nil,
	).Once()
	ipfsClient.On(
		"ImportToMFS",
		mock.Anything,
		mock.Anything,
		"/csi-volumes/test",
	).Return(errors.New("import failed")).Once()

	d := &Driver{
		logger:      zerolog.Nop(),
		cfg:         &Config{},
		ipfsClient:  ipfsClient,
		volumeStore: volumeStore,
	}

	_, err := d.NodeUnstageVolume(
		context.Background(), &csi.NodeUnstageVolumeRequest{
			VolumeId:          "vol-a",
			StagingTargetPath: t.TempDir(),
		},
	)
	if status.Code(err) != codes.Internal {
		t.Fatalf("NodeUnstageVolume() code = %v, want %v", status.Code(err), codes.Internal)
	}
}

func TestBuildMountOptions_IncludesReadOnlyAndMountFlags(t *testing.T) {
	t.Parallel()

	opts := buildMountOptions(
		&csi.NodePublishVolumeRequest{
			Readonly: true,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						MountFlags: []string{"nodev", "nosuid"},
					},
				},
			},
		},
	)

	want := []string{"bind", "ro", "nodev", "nosuid"}
	if len(opts) != len(want) {
		t.Fatalf("buildMountOptions() len = %d, want %d", len(opts), len(want))
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Fatalf("buildMountOptions()[%d] = %q, want %q", i, opts[i], want[i])
		}
	}
}

func TestBuildMountOptions_DeduplicatesFlags(t *testing.T) {
	t.Parallel()

	opts := buildMountOptions(
		&csi.NodePublishVolumeRequest{
			Readonly: true,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						MountFlags: []string{"ro", "bind", "noexec", "noexec", ""},
					},
				},
			},
		},
	)

	want := []string{"bind", "ro", "noexec"}
	if len(opts) != len(want) {
		t.Fatalf("buildMountOptions() len = %d, want %d", len(opts), len(want))
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Fatalf("buildMountOptions()[%d] = %q, want %q", i, opts[i], want[i])
		}
	}
}
