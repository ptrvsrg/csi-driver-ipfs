# Helm charts

| Chart             | Description                   |
|-------------------|-------------------------------|
| `csi-driver-ipfs` | CSI driver for IPFS volumes   |
| `ipfs-cluster`    | IPFS Cluster (Kubo + cluster) |

## Commands

Run `make -C charts help` — targets are grouped by `<group>/<target>`.

### Test

- `make -C charts test/all` — [helm-unittest](https://github.com/helm-unittest/helm-unittest) for all charts.
- `make -C charts lint/all` — `helm lint` on each chart.
- `make -C charts test/<chart>` — tests for one chart (for example `test/csi-driver-ipfs`).
- `make -C charts lint/<chart>` — lint one chart (for example `lint/ipfs-cluster`).

### Release

- `make -C charts release/package/all` — [chart-releaser](https://github.com/helm/chart-releaser) `cr package` -> `dist/`.
- `make -C charts release/upload/all` — `cr upload` to GitHub Releases (`CR_TOKEN` or `GITHUB_TOKEN`).
- `make -C charts release/index` — `cr index --push` for `gh-pages` (manual flow).
- `make -C charts clean` — remove `dist/` and downloaded `bin/cr`.

### Per-chart targets (dynamic)

`CHARTS` in `Makefile` drives generated targets via `define` / `foreach` / `eval`:

`test/<chart>`, `lint/<chart>`, `release/package/<chart>`, `release/upload/<chart>`.

Examples:

```bash
make -C charts test/csi-driver-ipfs
make -C charts lint/ipfs-cluster
make -C charts release/package/ipfs-cluster
make -C charts help
```

To add a new chart, put its directory name in `CHARTS := ...` in `Makefile`.

### chart-releaser (`cr`)

- Config: [`cr.yaml`](cr.yaml) (`owner`, `git-repo`, `package-path`).
- **Local packaging:** `make -C charts release/package/all` (no token).
- **Upload to GitHub Releases:**
  `CR_TOKEN=$(gh auth token) make -C charts release/upload/all`
  or set `GITHUB_TOKEN`. Uses `cr upload --skip-existing`.
- **Helm repo index (GitHub Pages):** set `charts-repo-url` in `cr.yaml` if needed, then `CR_TOKEN=... make -C charts release/index`.

Examples:

```bash
make -C charts lint/all && make -C charts test/all && make -C charts release/package/all
make -C charts test/all
CR_TOKEN=$(gh auth token) make -C charts release/upload/all
```

Unit tests live under each chart in `tests/*_test.yaml` and are excluded from packaged charts (see `.helmignore`).
