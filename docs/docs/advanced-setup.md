# Advanced Setup

## IPFS API endpoint configuration

The CSI driver connects to Kubo API using multiaddr-style endpoint configuration.
Ensure your `ipfsApi` value points to reachable service DNS and port.

Example pattern:

```text
/dns4/<service>.<namespace>.svc.cluster.local/tcp/5001
```

## Snapshot components strategy

For predictable behavior in test/prod:

- install snapshot CRDs explicitly,
- run a single cluster-level `snapshot-controller`,
- avoid deploying duplicate snapshot controllers from multiple charts.

## Security and RBAC hardening

- keep controller and node service accounts scoped to required verbs/resources,
- avoid broad wildcard RBAC rules,
- verify snapshot-controller has cluster-scope access to snapshot resources.

## Image management

- use immutable image tags for release environments,
- keep `latest` only for local/dev workflows,
- pin chart images and snapshot-controller versions.

## Helm repository setup

Single domain with split path:

- docs: `https://ptrvsrg.github.io/csi-driver-ipfs/`
- charts: `https://ptrvsrg.github.io/csi-driver-ipfs/charts/index.yaml`
