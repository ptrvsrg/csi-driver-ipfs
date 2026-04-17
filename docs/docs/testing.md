# Testing

## Unit tests

Run all unit tests:

```bash
make test/unit
```

Run with race detector:

```bash
make test/unit-race
```

## Chart tests

```bash
make -C charts lint/all
make -C charts test/all
```

## End-to-end tests

```bash
make -C test/e2e env/up
make -C test/e2e test/e2e
make -C test/e2e env/down
```

Focused suites:

```bash
make -C test/e2e test/ginkgo GINKGO_FOCUS=Provisioning
make -C test/e2e test/ginkgo GINKGO_FOCUS=Snapshot
make -C test/e2e test/kuttl
```

## What E2E covers

- dynamic provisioning,
- read-only volume flow,
- expansion behavior,
- snapshot create/restore,
- control-plane assertions via KUTTL.

## Debug strategy

Framework includes retry-aware diagnostics dump for failed operations:

- pods, PVC/PV, snapshots,
- namespace events,
- controller logs.
