# Contributing to csi-driver-ipfs

Thank you for your interest in contributing. This document explains how to submit changes and what we expect from
contributors.

## Code of Conduct

This project adheres to the [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this
code.

## Developer Certificate of Origin (DCO)

This project uses the [Developer Certificate of Origin (DCO)](DCO). By contributing, you agree to the terms of the DCO.
You certify that you have the right to submit your contribution under the project's license (Apache 2.0).

**Sign-off:** Each commit must include a sign-off line:

```text
Signed-off-by: Your Name <your.email@example.com>
```

Use `git commit -s` to add the sign-off automatically.

## How to Contribute

### Reporting Bugs and Suggesting Features

- **Bugs:** Open an [issue](https://github.com/ptrvsrg/csi-driver-ipfs/issues) using the bug report template. Include
  steps to reproduce, environment (Kubernetes version, IPFS setup), and logs if relevant.
- **Features:** Open an issue with the feature request template and describe the use case and proposed behavior.

### Pull Requests

1. **Fork** the repository and create a branch from `main`.
2. **Make your changes:** Follow the style and conventions used in the project (see below).

3. **Run checks locally:**

    ```bash
    make deps/all
    make verify/fmt
    make verify/lint
    make test/unit
    make -C charts lint/all
    make -C charts test/all
    ```

4. **Commit** with a clear subject and body. Prefer this shape: subject line `[<type>]: <summary>` (for example
  `feat`, `fix`, `docs`, `chore`, `test`, `ci`, `refactor`), a blank line, a short description, a blank line, then
  `Signed-off-by:` (use `git commit -s` for the sign-off line). Merge commits and subjects starting with `Revert` are
  fine as produced by Git.
5. **Push** and open a Pull Request. Fill in the PR template and link any related issues.

### Code Style and Quality

- **Formatting:** Run `make verify/fmt` (or `go fmt ./...`). The project uses `golangci-lint`; run `make verify/lint`
  and fix any reported issues. Use `make verify/lint-fix` for auto-fixable linters.
- **Tests:** Add or update tests for behavioral changes. Run `make test/unit` and, if applicable, `make test/unit-race`.
- **Helm charts:** Use `make -C charts` (see [charts/README.md](charts/README.md)): e.g. `make -C charts lint/all`,
  `make -C charts test/all`, `make -C charts release/package/all`, `CR_TOKEN=... make -C charts release/upload/all`.
- **License header:** New source files must include the Apache 2.0 license header. Use `make gen/license-header` to add
  it, and `make verify/license-header` to check.

### Project Structure

- `cmd/csi-driver-ipfs/` — main entrypoint; wires config, informers, and the CSI driver.
- `pkg/driver/` — CSI driver implementation (controller, node, identity).
- `pkg/ipfs/` — IPFS/Kubo API client.
- `pkg/kubernetes/` — Kubernetes informers (PV, VolumeSnapshotContent) and store interfaces; no direct API polling.
- `pkg/logging/`, `pkg/grpc/` — logging and gRPC middleware.
- `charts/` — Helm charts (csi-driver-ipfs, ipfs-cluster).
- `docs/` — Docusaurus documentation source and docs build toolchain.
- `.markdownlint.json` — rule overrides; **`.markdownlint-cli2.jsonc`** — globs and ignores for `markdownlint-cli2` (it
  does **not** read `.markdownlintignore`; that file is only for legacy `markdownlint-cli`).
- `hack/` — license-header scripts.
- `hack/ci/` — shell scripts used only from GitHub Actions (see `hack/ci/README.md`).

All library code lives under `pkg/`. An `internal/` package is not used so that the driver and helpers remain importable
if needed.

### Maintainers

See [MAINTAINERS.md](MAINTAINERS.md) for the list of maintainers and how to reach them.

Thank you for contributing.
