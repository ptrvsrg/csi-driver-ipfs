# Snapshot and Restore

This page describes snapshot and restore flow for `csi-driver-ipfs`.

## Prerequisites

- snapshot CRDs installed in the cluster,
- running `snapshot-controller`,
- `VolumeSnapshotClass` configured with driver:
  - `ipfs.csi.ptrvsrg.github.io`.

## Create snapshot

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: ipfs-snap
spec:
  volumeSnapshotClassName: ipfs-csi-snapclass
  source:
    persistentVolumeClaimName: ipfs-pvc
```

Wait until `status.readyToUse=true`.

## Restore from snapshot

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ipfs-restore-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ipfs-csi
  dataSource:
    name: ipfs-snap
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
```

## Driver behavior notes

- Controller reads `VolumeContentSource` from `CreateVolume` request.
- Snapshot source content is materialized into a new MFS path before volume publishing.
- Response includes original snapshot source in `Volume.ContentSource`.

## Failure modes

- `volume content source missing`: snapshot source is not propagated correctly.
- `forbidden` errors for snapshot controller: RBAC/controller deployment issue.
- `VolumeSnapshotClass not found`: class deleted too early or wrong class name.

## Validation

Use E2E suite:

```bash
make -C test/e2e test/ginkgo GINKGO_FOCUS=Snapshot
```
