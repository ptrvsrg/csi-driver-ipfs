# IPFS CSI drover for Kubernetes

[![CI DEV (Helm charts)](https://github.com/ptrvsrg/csi-driver-ipfs/actions/workflows/security.yml/badge.svg)](https://github.com/ptrvsrg/csi-driver-ipfs/actions/workflows/security.yml)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/csi-driver-ipfs)](https://artifacthub.io/packages/search?repo=csi-driver-ipfs)

## About

CSI driver that provisions Kubernetes PersistentVolumes on top of IPFS.

## Quick start (Helm)

1. Install snapshot CRDs and snapshot-controller (required for snapshot restore flows).
2. Deploy IPFS backend.
3. Deploy CSI driver chart.

Example:

```bash
helm upgrade --install ipfs-cluster charts/ipfs-cluster -n ipfs --create-namespace
helm upgrade --install csi-driver-ipfs charts/csi-driver-ipfs -n csi-ipfs --create-namespace
```

## Development

### What this repository contains

- `cmd/csi-driver-ipfs`: driver entrypoint and server bootstrap.
- `pkg/driver`: CSI Controller/Node/Identity implementations.
- `pkg/ipfs`: Kubo API client used by the driver.
- `charts/csi-driver-ipfs`: Helm chart for the CSI driver.
- `charts/ipfs-cluster`: Helm chart for single-cluster IPFS deployment used by local/dev scenarios.
- `test/e2e`: end-to-end suite (Ginkgo + KUTTL).
- `docs`: Docusaurus documentation site source.

### Build and unit tests

```bash
make build/golang
make test/unit
```

### Charts

```bash
make -C charts lint/all
make -C charts test/all
```

### E2E

```bash
make -C test/e2e env/up
make -C test/e2e test/e2e
make -C test/e2e env/down
```

## Documentation site

Local docs preview:

```bash
make -C docs dev/start
```

Published docs are expected on GitHub Pages, and packaged charts/index on the same domain under `/charts`.

## Helm repository path (GitHub Pages)

When GitHub Pages publication is enabled for this repository:

- Docs root: `https://ptrvsrg.github.io/csi-driver-ipfs/`
- Helm index: `https://ptrvsrg.github.io/csi-driver-ipfs/charts/index.yaml`

Use with Helm:

```bash
helm repo add csi-driver-ipfs https://ptrvsrg.github.io/csi-driver-ipfs/charts
helm repo update
```
