# Installation

## Prerequisites

- Kubernetes cluster with CSI support
- Helm
- IPFS backend (for example `ipfs-cluster` chart from this repository)
- VolumeSnapshot CRDs and `snapshot-controller` if you need snapshot restore

## Install snapshot CRDs (required for snapshot workflows)

```bash
TAG=v8.5.0
BASE="https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/${TAG}/client/config/crd"
kubectl apply -f "${BASE}/snapshot.storage.k8s.io_volumesnapshotclasses.yaml"
kubectl apply -f "${BASE}/snapshot.storage.k8s.io_volumesnapshotcontents.yaml"
kubectl apply -f "${BASE}/snapshot.storage.k8s.io_volumesnapshots.yaml"
```

Install `snapshot-controller` in `kube-system` (or equivalent) before testing restore operations.

## Deploy IPFS backend

```bash
helm upgrade --install ipfs-cluster charts/ipfs-cluster -n ipfs --create-namespace
```

## Deploy CSI driver chart

```bash
helm upgrade --install csi-driver-ipfs charts/csi-driver-ipfs -n csi-ipfs --create-namespace
```

## Verify deployment

```bash
kubectl -n csi-ipfs get pods
kubectl get storageclass
```

Expected result:

- controller and node components are Ready,
- `StorageClass` for the driver exists (default: `ipfs-csi`),
- snapshot CRDs are present if snapshot features are enabled.
