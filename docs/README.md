# Docusaurus docs

This directory contains the Docusaurus source for project documentation.

## Content structure

- `docs/intro.md` (root page)
- `docs/installation.md`, `docs/usage.md`, `docs/snapshot-and-restore.md`
- `docs/advanced-setup.md`, `docs/troubleshooting.md`
- `docs/architecture.md`, `docs/controller-service.md`, `docs/node-service.md`
- `docs/references.md`
- `docs/development.md`, `docs/testing.md`, `docs/ci-cd.md`

## Local usage

Install dependencies once (`yarn install --frozen-lockfile` — the `install` target runs this):

```bash
make start
```

## Build

```bash
make build
```

Static output is generated into `docs/build` and published by `publish-pages.yml`.
