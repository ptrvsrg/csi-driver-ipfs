# E2E test environment

Kind cluster with **ipfs-cluster** (Helm) and **csi-driver-ipfs** (Helm) from `charts/`. Targets are defined in *
*[../Makefile](../Makefile)** — invoke them with **`make -C test/e2e <target>`** from the repository root (or
`cd test/e2e && make`).

`env/up` also installs:

- external-snapshotter CRDs
- upstream `snapshot-controller` (required for VolumeSnapshot/restore flows)

## Up

From repo root:

```bash
make -C test/e2e env/up
```

Optional: build and load driver image, then re-deploy CSI:

```bash
make -C test/e2e env/load-image
make -C test/e2e env/deploy-csi
```

## Run e2e

```bash
make -C test/e2e test/e2e
```

## Down

```bash
make -C test/e2e env/down
```
