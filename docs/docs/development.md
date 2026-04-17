# Development

## Local prerequisites

- Go toolchain compatible with `go.mod`
- Docker and Kind for E2E
- Helm for chart testing
- Node.js (for docs site)

## Core quality checks

```bash
make deps/all
make verify/fmt
make verify/lint
make test/unit
```

## Chart checks

```bash
make -C charts lint/all
make -C charts test/all
```

## E2E checks

```bash
make -C test/e2e env/up
make -C test/e2e test/e2e
make -C test/e2e env/down
```

## Mocks and generated code

```bash
make gen/mocks
```

Mocks are generated from `.mockery.yaml` into package-specific `mocks` directories.

## Local docs

```bash
make -C docs start
```

## Recommended change flow

1. Make code changes.
2. Run unit and lints.
3. Run chart checks when touching charts/manifests.
4. Run focused E2E suites for behavior changes.
5. Update docs for any API/behavior/ops change.
