# Node Service

Node service implements CSI Node API in `pkg/driver/node.go`.

## Key methods

### `NodeStageVolume(ctx, req) (*csi.NodeStageVolumeResponse, error)`

- prepares staged path,
- fetches or materializes volume content locally,
- validates mount/input arguments.

### `NodePublishVolume(ctx, req) (*csi.NodePublishVolumeResponse, error)`

- publishes staged content into pod target path,
- supports mount semantics and read-only behavior.

### `NodeUnpublishVolume(ctx, req) (*csi.NodeUnpublishVolumeResponse, error)`

- unmounts and cleans target path.

### `NodeExpandVolume(ctx, req) (*csi.NodeExpandVolumeResponse, error)`

- performs node-side filesystem resize where required.

## Important behavior

- destination directories are cleaned before exporting content to avoid stale-file collisions.
- path handling is defensive to maintain idempotent publish flows across retries.

## Typical error classes

- invalid path/arguments,
- mount failures from node environment,
- IPFS fetch/export failures.
