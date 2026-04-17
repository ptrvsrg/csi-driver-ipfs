# References

## Driver identity

- CSI driver name: `ipfs.csi.ptrvsrg.github.io`

## Main paths

- Entry point: `cmd/csi-driver-ipfs`
- Driver logic: `pkg/driver`
- IPFS client: `pkg/ipfs`
- Kubernetes integration: `pkg/kubernetes`
- Helm charts: `charts/csi-driver-ipfs`, `charts/ipfs-cluster`
- E2E framework and suites: `test/e2e`

## Helm repository

- URL: `https://ptrvsrg.github.io/csi-driver-ipfs/charts`
- Index: `https://ptrvsrg.github.io/csi-driver-ipfs/charts/index.yaml`

## Common commands

```bash
make test/unit
make -C charts test/all
make -C test/e2e test/e2e
make -C docs build
```

## IPFS logo (documentation)

The IPFS cube mark on the docs home page and in the site header is
[File:Ipfs-logo-1024-ice-text.png](https://commons.wikimedia.org/wiki/File:Ipfs-logo-1024-ice-text.png) on Wikimedia Commons
([CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/)); original source [ipfs/logo](https://github.com/ipfs/logo).

## Related documentation

- Kubernetes CSI docs: [kubernetes-csi.github.io/docs](https://kubernetes-csi.github.io/docs/)
- External snapshotter docs: [external-snapshotter](https://kubernetes-csi.github.io/docs/external-snapshotter.html)
- Snapshot restore docs: [snapshot-restore-feature](https://kubernetes-csi.github.io/docs/snapshot-restore-feature.html)
