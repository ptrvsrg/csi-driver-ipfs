# Controller Service

Controller service implements the CSI Controller API in `pkg/driver/controller.go`.

## Key methods

### `CreateVolume(ctx, req) (*csi.CreateVolumeResponse, error)`

Responsibilities:

- validate request and capacity,
- process parameters and content source,
- create new backing object in IPFS,
- return CSI `Volume` with context and capabilities.

Snapshot case:

- reads `req.GetVolumeContentSource()`,
- resolves `snapshot_id`,
- restores snapshot content into a fresh MFS path.

### `DeleteVolume(ctx, req) (*csi.DeleteVolumeResponse, error)`

- idempotent delete behavior for already-removed volumes,
- cleanup of driver metadata and backing content references.

### `ControllerExpandVolume(ctx, req) (*csi.ControllerExpandVolumeResponse, error)`

- processes requested capacity increase,
- returns expansion result to resizer sidecar.

## Error handling

- uses gRPC status codes for invalid arguments, not found, and internal errors,
- preserves idempotency where CSI semantics require it.

## Integration points

- IPFS client for content operations,
- stores for PV and snapshot metadata lookup,
- Kubernetes snapshot API for restore context.
