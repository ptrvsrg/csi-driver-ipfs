# hack/ci

Shell scripts used only from **GitHub Actions** (or for local debugging of the same checks).

- `verify-chart-release-tag.sh` (used by `release-chart.yml`) validates that tag
  `chart/csi-driver-ipfs/v*` or `chart/ipfs-cluster/v*` matches `version` in `Chart.yaml`
  (parses `Chart.yaml` with `awk` — no `yq` required in CI).
- `verify-chart-version-bump.sh` (used by `ci-dev-charts.yml`) requires a bumped `version`
  in `Chart.yaml` whenever files under a chart directory change in a PR.

Run with `--help` for usage. Example:

```bash
GITHUB_REF=refs/tags/chart/csi-driver-ipfs/v0.1.0 REPO_ROOT="$(git rev-parse --show-toplevel)" \
  ./hack/ci/verify-chart-release-tag.sh
```
