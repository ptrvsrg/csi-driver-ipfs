# Kubernetes manifests (`deploy/k8s`)

## IPFS API (`ipfsApi`)

The CSI driver uses a [multiaddr](https://github.com/multiformats/multiaddr) for the Kubo HTTP API. Example for a single ipfs-cluster peer (see `v0.0.1/ipfs-cluster.yaml`):

```text
/dns4/demo-ipfs-cluster-0.demo-ipfs-cluster.ipfs.svc.cluster.local/tcp/5001
```

Adjust **StatefulSet name**, **headless Service**, and **namespace** to match your install. Deploy **ipfs-cluster** (or your IPFS Service) before CSI, or pods will fail to connect until the API exists.

## VolumeSnapshot CRDs (required for snapshots + controller informer)

Kind and many clusters do **not** ship `snapshot.storage.k8s.io` CRDs. Install them **before** or **with** the CSI driver if you use volume snapshots.

Align the tag with the [external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter) version used in the Helm chart (`csiSnapshotter` image tag, e.g. `v8.5.0`):

```bash
TAG=v8.5.0
BASE="https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/${TAG}/client/config/crd"
kubectl apply -f "${BASE}/snapshot.storage.k8s.io_volumesnapshotclasses.yaml"
kubectl apply -f "${BASE}/snapshot.storage.k8s.io_volumesnapshotcontents.yaml"
kubectl apply -f "${BASE}/snapshot.storage.k8s.io_volumesnapshots.yaml"
```

If CRDs are **not** installed, the driver **skips** the VolumeSnapshot informer (after discovery) and still starts the CSI socket.

## RBAC

The node `ServiceAccount` needs `list/watch` on `persistentvolumes` and `volumesnapshotcontents` for informers. This is included in the chart and in `v0.0.1/csi-driver-ipfs.yaml` (`ipfs-csi-node-role`).

## CSI socket / probes

The driver waits for informer cache sync before starting the CSI gRPC socket. Sidecar health checks require the Unix socket, so missing CRDs/RBAC can delay or block startup until fixed.
