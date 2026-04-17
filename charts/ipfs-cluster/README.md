# IPFS Cluster

Helm chart for deploying [IPFS Cluster](https://ipfscluster.io) on Kubernetes. Each replica runs an IPFS (Kubo) node and an ipfs-cluster-service peer; the first peer (ordinal 0) acts as bootstrap for the others.

## Prerequisites

- Kubernetes 1.21+
- Helm 3

## Install

Generate cluster secret and bootstrap identity for the first peer:

```bash
# Cluster secret (32 bytes hex)
export CLUSTER_SECRET=$(od -vN 32 -An -tx1 /dev/urandom | tr -d ' \n')

# Bootstrap peer ID and private key (install ipfs-key: go install github.com/whyrusleeping/ipfs-key@latest)
# ipfs-key outputs "peer_id base64_private_key"
export BOOTSTRAP_KEYS=$(ipfs-key)
export BOOTSTRAP_PEER_ID=$(echo $BOOTSTRAP_KEYS | cut -d' ' -f1)
export BOOTSTRAP_PRIV_KEY=$(echo $BOOTSTRAP_KEYS | cut -d' ' -f2)
```

Install the chart:

```bash
helm install ipfs-cluster ./charts/ipfs-cluster --namespace ipfs \
  --set clusterSecret="$CLUSTER_SECRET" \
  --set bootstrapPeerId="$BOOTSTRAP_PEER_ID" \
  --set bootstrapPeerPrivKey="$BOOTSTRAP_PRIV_KEY"
```

Or use an existing Secret that contains `cluster-secret` and `bootstrap-peer-priv-key`:

```bash
helm install ipfs-cluster ./charts/ipfs-cluster --namespace ipfs \
  --set existingSecret=my-ipfs-cluster-secret \
  --set bootstrapPeerId="<first-peer-id>"
```

(The bootstrap peer ID is stored in a ConfigMap; the private key must come from the Secret.)

## Configuration

| Parameter                 | Description                                                          | Default             |
|---------------------------|----------------------------------------------------------------------|---------------------|
| `replicas`                | Number of cluster peers                                              | `3`                 |
| `clusterSecret`           | Cluster secret (raw string, required when `existingSecret` is empty) | `""`                |
| `existingSecret`          | Use existing Secret for cluster-secret and bootstrap key             | `""`                |
| `bootstrapPeerId`         | First peer's ID (from ipfs-key)                                      | `""`                |
| `bootstrapPeerPrivKey`    | First peer's private key (base64)                                    | `""`                |
| `ipfsImage.repository`    | Kubo image                                                           | `ipfs/kubo`         |
| `ipfsImage.tag`           | Kubo tag                                                             | `v0.32.1`           |
| `clusterImage.repository` | IPFS Cluster image                                                   | `ipfs/ipfs-cluster` |
| `clusterImage.tag`        | IPFS Cluster tag                                                     | `v1.0.3`            |
| `storage.cluster.size`    | Size for cluster state PVC                                           | `5Gi`               |
| `storage.ipfs.size`       | Size for IPFS repo PVC                                               | `50Gi`              |

## Integration with CSI Driver

After installing this chart, point the [CSI driver](../csi-driver-ipfs) to the cluster:

- **Controller / Node IPFS API**: Use the cluster API proxy or a specific peer's IPFS API, e.g.
  `http://<release-name>-api.<namespace>.svc.cluster.local:9094` (proxy) or
  `/dns4/<release-name>-0.<release-name>.<namespace>.svc.cluster.local/tcp/5001` for peer-0's IPFS API.

## Uninstall

```bash
helm uninstall ipfs-cluster -n ipfs
```

Note: PVCs are not deleted by default; remove them manually if needed.
