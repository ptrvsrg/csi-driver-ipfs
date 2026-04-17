# CSI Driver for IPFS

Helm chart for deploying the [CSI IPFS Driver](https://github.com/ptrvsrg/csi-driver-ipfs) on Kubernetes.

## Prerequisites

- Kubernetes 1.28+
- Helm 3
- IPFS node or [IPFS Cluster](../ipfs-cluster)
- **VolumeSnapshot CRDs** if you use snapshots: see [deploy/k8s/README.md](../../deploy/k8s/README.md). Without CRDs, snapshot informers are skipped (discovery); install CRDs for full snapshot support.

## Install

```bash
# Add repo (when published) or install from local path
helm install csi-driver-ipfs ./charts/csi-driver-ipfs --namespace kube-system

# With custom driver image
helm install csi-driver-ipfs ./charts/csi-driver-ipfs --namespace kube-system \
  --set image.driver.repository=ghcr.io/ptrvsrg/csi-driver-ipfs \
  --set image.driver.tag=0.1.0
```

## Configuration

| Parameter                     | Description                                                      | Default                           |
|-------------------------------|------------------------------------------------------------------|-----------------------------------|
| `driver.name`                 | CSI driver name (must match `--drivername`)                      | `ipfs.csi.ptrvsrg.github.io`      |
| `image.driver.repository`     | CSI driver image                                                 | `ghcr.io/ptrvsrg/csi-driver-ipfs` |
| `image.driver.tag`            | Image tag                                                        | `latest`                          |
| `ipfsApi`                     | IPFS API multiaddr (controller + node unless `node.ipfsApi` set) | DNS example in `values.yaml`      |
| `node.ipfsApi`                | Optional: override IPFS API multiaddr for node DaemonSet only    | `""` (use `ipfsApi`)              |
| `controller.replicas`         | Controller replicas                                              | `1`                               |
| `controller.hostNetwork`      | Run controller pod in host network namespace                     | `false`                           |
| `externalSnapshotter.enabled` | Deploy snapshot controller                                       | `false`                           |
| `storageClass.create`         | Create default StorageClass                                      | `false`                           |

When using with [ipfs-cluster](../ipfs-cluster), set `ipfsApi` and `node.ipfsApi` to the cluster API (e.g. `/dns4/ipfs-cluster-api/tcp/9094` or the proxy endpoint).

### Health probes (`hostNetwork`)

The `csi-livenessprobe` sidecar **does not open** its HTTP server until the CSI Unix socket exists and `GetDriverName` succeeds (upstream behavior). Until then, `connection refused` is expected; **`startupProbe`** on the driver and registrar tolerates this (up to `failureThreshold` × `periodSeconds`, default 5 minutes).

Sidecars use `--http-endpoint=:PORT` (listen on all interfaces). Probes omit `httpGet.host` so kubelet uses the **Pod IP**.

If the **controller** cannot reach IPFS on the default address, set `ipfsApi` to a reachable multiaddr (Kubernetes DNS is recommended: `/dns4/<pod>.<svc>.<ns>.svc.cluster.local/tcp/5001`).

### StorageClass parameters

The driver recognizes `cid`, `mfspath`, and `pinning` only. Other keys may be persisted in the PV context but are ignored by the driver logic.

## Uninstall

```bash
helm uninstall csi-driver-ipfs -n kube-system
```
