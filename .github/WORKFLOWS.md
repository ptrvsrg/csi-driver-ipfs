# GitHub Actions workflows

## Job dependency graph (high level)

| Workflow                    | Job order                                                                                                                        |
|-----------------------------|----------------------------------------------------------------------------------------------------------------------------------|
| **CI DEV (Go/Docker)**      | (`mod-verify` ∥ `go-fmt` ∥ `go-vet` ∥ `check-license` ∥ `goreleaser-check`) → (`lint` ∥ `unit`) → `build` → `docker-build-push`  |
| **CI DEV (Helm charts)**    | `chart-version` -> `lint` -> `test` -> `package`                                                                                 |
| **CI DEV (markdown)**       | `lint`                                                                                                                           |
| **CI DEV (docs site)**      | `build` (Docusaurus)                                                                                                             |
| **CI DEV (GitHub Actions)** | `actionlint`                                                                                                                     |
| **Publish Pages**           | `docs-build` + `charts-build` -> `pages-assemble` -> `pages-deploy`                                                              |
| **Release (Helm charts)**   | `verify-version` -> `publish`                                                                                                    |
| **Release (Golang)**        | `release`                                                                                                                        |
| **E2E tests**               | PR label `e2e/run` → `remove-label` → `check-image` → `e2e`; then `e2e/success` or `e2e/fail`                                    |
| **Security scans**          | PR label `security/run` → `remove-label` → scan jobs → `security/success` or `security/fail`                                     |

## Workflow index

| File                  | Trigger                                                  | Purpose                                                                                        |
|-----------------------|----------------------------------------------------------|------------------------------------------------------------------------------------------------|
| `ci-dev-golang.yml`   | `pull_request` (path-filtered)                           | Go checks, `goreleaser check`, golangci-lint, tests, dev image to ghcr (same-repo PRs)         |
| `ci-dev-charts.yml`   | `pull_request` (path-filtered)                           | Helm lint, helm-unittest, chart packaging artifacts                                            |
| `ci-dev-markdown.yml` | `pull_request` (path-filtered)                           | Markdown lint when matching `**/*.md` or markdownlint config                                   |
| `ci-dev-docs.yml`     | `pull_request` (path-filtered)                           | Docusaurus build for `docs/**` changes                                                         |
| `ci-dev-actions.yml`  | `pull_request` (path-filtered)                           | `actionlint` on `.github/workflows/**` (and `.github/Makefile` when workflows tooling changes) |
| `e2e.yml`             | `pull_request` with `labeled` event                      | E2E in Kind using Ginkgo suites                                                                |
| `release-golang.yml`  | semver tag `v*` (e.g. `v1.2.3`) or manual                | Build and publish binaries + image via GoReleaser                                              |
| `release-chart.yml`   | tags `chart/csi-driver-ipfs/v*`, `chart/ipfs-cluster/v*` | Verify tag/version and publish chart release assets                                            |
| `publish-pages.yml`   | push on `main` or manual                                 | Build docs and chart index, publish to GitHub Pages                                            |
| `labeler.yml`         | PR opened/sync                                           | Path-based labels (`actions/labeler`) + size labels (`CodelyTV/pr-size-labeler`)               |
| `security.yml`        | `push` on `main`/tags, `pull_request`                    | Security scans (Trivy, CodeQL, Semgrep, Checkov, ZAP, Polaris)                                 |

## Path filters (`ci-dev-*.yml`)

Each `ci-dev-*.yml` workflow lists `paths:` under `pull_request` so it runs only when matching files change. Patterns are **GitHub Actions globs** (minimatch), not regular expressions — see [workflow syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#onpull_request).

## Local validation

- **actionlint:** `make -C .github verify/actionlint` (installs `actionlint` into `.github/bin/` via `go install`). Override version: `make -C .github deps/actionlint ACTIONLINT_VERSION=v1.7.7`.

## Caching and artifacts

- **Go:** `setup-go` with `cache-dependency-path: go.sum` in jobs that use Go.
- **Node:** `setup-node` with `cache: yarn` and `cache-dependency-path: docs/yarn.lock` for Docusaurus builds in CI.
- **Artifacts:** chart packages (`ci-dev-charts`), Pages build payload (`publish-pages`), coverage/binary from dev workflows as configured in each file.

## Tags

- **Image / Go release:** push git tag **`v1.2.3`** (or `v1.2.3-rc.1`, …); binaries and container tags follow the same semver naming (see `.goreleaser.yaml`).
- **Charts:** `chart/csi-driver-ipfs/v0.1.0` or `chart/ipfs-cluster/v0.1.0` — version must match `Chart.yaml`.

## GitHub Pages layout

Single domain, two paths:

- Docs site: `https://<owner>.github.io/<repo>/`
- Helm repo/index: `https://<owner>.github.io/<repo>/charts/index.yaml`

This enables Artifact Hub-friendly chart index URL while keeping docs on the same GitHub Pages domain.
