# hack

Development scripts for this repository.

## CI scripts (`hack/ci/`)

Scripts invoked from GitHub Actions are under **[ci/](ci/README.md)** (e.g. chart tag validation).

## Scripts

| Script                       | Purpose                                                             |
| ---------------------------- | ------------------------------------------------------------------- |
| `generate-license-header.sh` | Add Apache 2.0 license headers (used by `make gen-license-header`). |
| `validate-license-header.sh` | Verify license headers (used by `make validate-license-header`).    |
