# Troubleshooting

## Snapshot restore does not start

Symptoms:

- PVC from snapshot remains Pending,
- provisioner events mention source/content errors.

Checks:

1. `kubectl get volumesnapshot -A` and verify `readyToUse`.
2. `kubectl get volumesnapshotclass`.
3. verify `dataSource` references `VolumeSnapshot` and correct `apiGroup`.
4. inspect controller logs in `csi-ipfs` namespace.

## Snapshot-controller RBAC errors

Symptoms:

- logs contain `forbidden` for `volumesnapshots`, `volumesnapshotcontents`, or `persistentvolumes`.

Fix:

- ensure official RBAC manifest is applied,
- ensure service account used by snapshot-controller is bound to cluster roles.

## Volume publish fails with stale files

Symptoms:

- mount/publish operations fail due to existing content in destination paths.

Fix:

- validate node plugin is updated to current logic that clears destination directories before import/export.

## E2E cluster issues

Symptoms:

- tests fail to connect to cluster or use wrong kube context.

Fix:

- run `make -C test/e2e env/up`,
- verify `kubectl config current-context` is `kind-csi-ipfs-e2e`,
- re-run focused suite with `GINKGO_FOCUS`.

## Docs build fails

Run:

```bash
yarn --cwd docs install --frozen-lockfile
yarn --cwd docs run build
```

If broken links are reported, fix sidebar/navbar links and root doc slug.
