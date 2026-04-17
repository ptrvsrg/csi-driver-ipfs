# Architecture

## Motivation

`csi-driver-ipfs` brings IPFS-backed storage into standard Kubernetes CSI flows.
Applications use normal PVC/PV semantics, while content is persisted in IPFS.

## Components

- **CSI Controller (`pkg/driver/controller.go`)**
  - Create/Delete/Expand volume operations,
  - snapshot source handling in `CreateVolume`.
- **CSI Node (`pkg/driver/node.go`)**
  - stage/publish operations,
  - filesystem operations and content materialization.
- **IPFS Client (`pkg/ipfs/client.go`)**
  - wraps Kubo API interactions.
- **Kubernetes informers (`pkg/kubernetes`)**
  - watches PV and snapshot content resources for state correlation.

## Control-plane integration

Driver relies on Kubernetes CSI sidecars and snapshot ecosystem:

- `external-provisioner`
- `external-resizer`
- `csi-snapshotter`
- `snapshot-controller` (cluster-level)

## Data model concepts

- **Volume ID**: CSI identity for provisioned volume.
- **CID**: immutable content identifier in IPFS.
- **MFS path**: mutable path used by controller/node workflows.
- **Snapshot source**: CSI `VolumeContentSource` with `snapshot_id`.

## Request lifecycle (high-level)

1. PVC appears.
2. Provisioner invokes `CreateVolume`.
3. Controller allocates/links content path in IPFS.
4. Node stages/publishes content to pod path.
5. Optional expand/snapshot/restore flows update content lifecycle.
