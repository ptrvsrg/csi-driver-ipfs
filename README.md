# csi-driver-ipfs

CSI driver that provisions Kubernetes PersistentVolumes on top of IPFS.

## What this repository contains

- `cmd/csi-driver-ipfs`: driver entrypoint and server bootstrap.
- `pkg/driver`: CSI Controller/Node/Identity implementations.
- `pkg/ipfs`: Kubo API client used by the driver.
- `charts/csi-driver-ipfs`: Helm chart for the CSI driver.
- `charts/ipfs-cluster`: Helm chart for single-cluster IPFS deployment used by local/dev scenarios.
- `test/e2e`: end-to-end suite (Ginkgo + KUTTL).
- `docs`: Docusaurus documentation site source.

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
make -C docs start
```

Published docs are expected on GitHub Pages, and packaged charts/index on the same domain under `/charts`.

## Helm repository path (GitHub Pages)

When GitHub Pages publication is enabled for this repository:

- Docs root: `https://<owner>.github.io/<repo>/`
- Helm index: `https://<owner>.github.io/<repo>/charts/index.yaml`

Use with Helm:

```bash
helm repo add csi-driver-ipfs https://<owner>.github.io/<repo>/charts
helm repo update
```
