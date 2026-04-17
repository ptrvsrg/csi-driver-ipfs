// Copyright 2026 ptrvsrg.
//
// Licensed under the Apache License, Version 2.0 (the "License");
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

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetPluginInfo implements csi.IdentityServer and returns the driver name and vendor version.
//
// ctx is the request context. req is the CSI request (typically unused).
// On success it returns GetPluginInfoResponse with Name and VendorVersion; if the driver name is
// not configured it returns nil and an Unavailable gRPC status.
func (d *Driver) GetPluginInfo(
	_ context.Context,
	_ *csi.GetPluginInfoRequest,
) (*csi.GetPluginInfoResponse, error) {
	d.logger.Debug().Msg("get plugin info")

	if d.cfg.DriverName == "" {
		d.logger.Error().Msg("driver name not configured")
		return nil, status.Error(codes.Unavailable, "driver name not configured")
	}

	return &csi.GetPluginInfoResponse{
		Name:          d.cfg.DriverName,
		VendorVersion: d.cfg.Version,
	}, nil
}

// GetPluginCapabilities implements csi.IdentityServer and reports controller service support.
//
// ctx and req follow the CSI contract; req is unused.
// It returns a non-nil GetPluginCapabilitiesResponse listing CONTROLLER_SERVICE, or an error on failure.
func (d *Driver) GetPluginCapabilities(
	_ context.Context,
	_ *csi.GetPluginCapabilitiesRequest,
) (*csi.GetPluginCapabilitiesResponse, error) {
	d.logger.Debug().Msg("get plugin capabilities")

	caps := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}

	return caps, nil
}

// Probe implements csi.IdentityServer and reports readiness based on IPFS API connectivity (Ping).
//
// ctx cancels the health check. The ProbeRequest is unused.
// It returns Ready=true when Ping succeeds, otherwise Ready=false, without failing the RPC solely
// because IPFS is down (Probe still succeeds at the transport level).
func (d *Driver) Probe(ctx context.Context, _ *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	d.logger.Debug().Msg("probe")

	ready := true
	if err := d.ipfsClient.Ping(ctx); err != nil {
		d.logger.Warn().Err(err).Msg("ping IPFS server")
		ready = false
	}

	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: ready,
		},
	}, nil
}
