---
slug: /
---

# Overview

`csi-driver-ipfs` is a Kubernetes CSI driver that persists volume data in IPFS and exposes it through standard PVC/PV workflows.

## Purpose

This project exists to provide a CSI-compatible storage integration for IPFS workloads with:

- dynamic provisioning through Kubernetes `StorageClass`,
- volume expansion support,
- snapshot and restore support,
- Helm-based installation and reproducible E2E validation.

## Documentation map

### Getting Started

- [Installation](installation.md)
- [Usage](usage.md)
- [Snapshot and Restore](snapshot-and-restore.md)

### Administration Guides

- [Advanced Setup](advanced-setup.md)
- [Troubleshooting](troubleshooting.md)

### Internals

- [Architecture](architecture.md)
- [Controller Service](controller-service.md)
- [Node Service](node-service.md)

### References

- [References](references.md)

### Development

- [Development](development.md)
- [Testing](testing.md)
- [CI/CD](ci-cd.md)
