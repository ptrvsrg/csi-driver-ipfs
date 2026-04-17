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
	ipfsmocks "github.com/ptrvsrg/csi-driver-ipfs/pkg/ipfs/mocks"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewDriver_Validation(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)

	_, err := NewDriver(
		zerolog.Nop(),
		&Config{NodeID: "node-1"},
		ipfsClient,
		volumeStore,
		nil,
	)
	if !errors.Is(err, ErrEmptyDriverName) {
		t.Fatalf("expected ErrEmptyDriverName, got %v", err)
	}

	_, err = NewDriver(
		zerolog.Nop(),
		&Config{DriverName: DefaultDriverName},
		ipfsClient,
		volumeStore,
		nil,
	)
	if !errors.Is(err, ErrEmptyNodeID) {
		t.Fatalf("expected ErrEmptyNodeID, got %v", err)
	}
}

func TestIdentityServer(t *testing.T) {
	t.Parallel()

	ipfsClient := ipfsmocks.NewMockClient(t)
	volumeStore := storemocks.NewMockVolumeStore(t)
	snapshotStore := storemocks.NewMockSnapshotStore(t)

	d, err := NewDriver(
		zerolog.Nop(),
		&Config{
			DriverName: DefaultDriverName,
			NodeID:     "node-a",
			Version:    "v-test",
		},
		ipfsClient,
		volumeStore,
		snapshotStore,
	)
	if err != nil {
		t.Fatalf("NewDriver() error = %v", err)
	}

	info, err := d.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
	if err != nil {
		t.Fatalf("GetPluginInfo() error = %v", err)
	}
	if info.GetName() != DefaultDriverName {
		t.Fatalf("GetPluginInfo().Name = %q, want %q", info.GetName(), DefaultDriverName)
	}
	if info.GetVendorVersion() != "v-test" {
		t.Fatalf("GetPluginInfo().VendorVersion = %q, want %q", info.GetVendorVersion(), "v-test")
	}

	caps, err := d.GetPluginCapabilities(context.Background(), &csi.GetPluginCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetPluginCapabilities() error = %v", err)
	}
	if len(caps.GetCapabilities()) == 0 {
		t.Fatalf("GetPluginCapabilities() returned no capabilities")
	}
}

func TestProbe_Readiness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pingErr   error
		wantReady bool
	}{
		{
			name:      "ready when ping succeeds",
			pingErr:   nil,
			wantReady: true,
		},
		{
			name:      "not ready when ping fails",
			pingErr:   errors.New("ipfs unavailable"),
			wantReady: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ipfsClient := ipfsmocks.NewMockClient(t)
			volumeStore := storemocks.NewMockVolumeStore(t)
			ipfsClient.On("Ping", mock.Anything).Return(tt.pingErr).Once()

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

			resp, err := d.Probe(context.Background(), &csi.ProbeRequest{})
			if err != nil {
				t.Fatalf("Probe() error = %v", err)
			}
			if resp.GetReady().GetValue() != tt.wantReady {
				t.Fatalf("Probe().Ready = %v, want %v", resp.GetReady().GetValue(), tt.wantReady)
			}
		})
	}
}

func TestGetPluginInfo_EmptyDriverName(t *testing.T) {
	t.Parallel()

	d := &Driver{
		logger: zerolog.Nop(),
		cfg: &Config{
			DriverName: "",
		},
		volumeStore: &storeMock{},
	}

	_, err := d.GetPluginInfo(context.Background(), &csi.GetPluginInfoRequest{})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("GetPluginInfo() code = %v, want %v", status.Code(err), codes.Unavailable)
	}
}

type storeMock struct{}

func (s *storeMock) GetVolumeAttributes(context.Context, string) (*store.VolumeAttributes, error) {
	return nil, nil
}

func (s *storeMock) ListVolumes(context.Context, string) ([]*store.VolumeAttributes, error) {
	return nil, nil
}
