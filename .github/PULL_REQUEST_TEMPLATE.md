## Description

Brief description of the changes in this PR.

## Type of change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would change existing behavior)
- [ ] Documentation or chore

## Related issues

Fixes #(issue number) (if applicable).

## Checklist

General (see [CONTRIBUTING.md](../CONTRIBUTING.md) for details):

- [ ] Code style: `make verify/fmt`, `make verify/vet`, and `make verify/lint` pass (or `make verify/all` for the full static set).
- [ ] Dependencies/tools: run `make deps/all` when you first set up or when tool versions change.
- [ ] Tests: `make test/unit` (and `make test/unit-race` if you touch concurrency-sensitive code); add or update tests for behavior changes.
- [ ] License headers: `make verify/license-header` passes for touched source files.
- [ ] DCO: commits include a sign-off (`git commit -s`).
- [ ] Docs: user-facing behavior is reflected in `docs/` or chart READMEs where needed.

Scope-specific (run what applies to your change):

- [ ] **Helm (`charts/**`):** bump `version` in each affected chart’s `Chart.yaml`; `make -C charts lint/all` and `make -C charts test/all` pass.
- [ ] **Docs site (`docs/**`):** `make -C docs build` succeeds after `yarn install` (see [docs/README.md](../docs/README.md)).
- [ ] **GitHub Actions (`.github/workflows/**` or `.github/Makefile`):** `make -C .github verify/actionlint` passes.
- [ ] **GoReleaser (`.goreleaser.yaml`):** `goreleaser check` passes (install via `go install github.com/goreleaser/goreleaser/v2@<version>` or use the version pinned in `ci-dev-golang.yml`).
