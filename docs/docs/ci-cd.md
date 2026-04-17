# CI/CD

## Pipelines

Dev PR workflows (`ci-dev-*.yml`) use `pull_request` with `paths` globs so they run only when relevant files change (GitHub glob syntax, not regex).

- `ci-dev-golang.yml`:
  - (`mod-verify` ∥ `go-fmt` ∥ `go-vet` ∥ `check-license` ∥ `goreleaser-check`) → (`lint` ∥ `unit`) → `build` → `docker-build-push`
- `ci-dev-charts.yml`:
  - `chart-version` -> `lint` -> `test` -> `package` (if anything under `charts/<name>/` changes, `Chart.yaml` `version` must differ from the PR base)
- `ci-dev-markdown.yml`: markdownlint when `**/*.md` or markdownlint config changes.
- `ci-dev-docs.yml`: Docusaurus `yarn run build` under `docs/` when `docs/**` changes.
- `ci-dev-actions.yml`: `actionlint` via `make -C .github verify/actionlint` when `.github/workflows/**` or `.github/Makefile` changes.
- `release-golang.yml`: image and binary release via GoReleaser; git tags are `driver/v*`, artifact and image versions are semver **without** the `driver/` prefix (e.g. tag `driver/v1.0.0` → `v1.0.0`).
- `release-chart.yml`: chart release flow for `chart/<name>/v<version>` tags.
- `publish-pages.yml`: publishes Docusaurus and chart index to GitHub Pages.

## GitHub Pages layout

Single Pages domain with split paths:

- Docs site root: `https://ptrvsrg.github.io/csi-driver-ipfs/`
- Chart repository: `https://ptrvsrg.github.io/csi-driver-ipfs/charts/index.yaml`

This allows Artifact Hub registration with chart index URL while keeping docs on the same domain.

## Release conventions

- Git tags for releases use `driver/v*`; published `main.driverVersion` and OCI tags use the suffix only (e.g. `v1.0.0`).
- Chart tags use:
  - `chart/csi-driver-ipfs/vX.Y.Z`
  - `chart/ipfs-cluster/vX.Y.Z`

Version in tag must match `Chart.yaml`.

## What to update when behavior changes

- docs under `docs/docs/`,
- chart docs in `charts/*/README.md`,
- CI docs in `.github/README.md` when workflow logic changes.
