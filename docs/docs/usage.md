# Usage

## Dynamic provisioning (RWO)

Create a PVC using the `ipfs-csi` storage class from the chart defaults:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: ipfs-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ipfs-csi
```

Attach the claim to a pod and write data:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ipfs-app
spec:
  containers:
    - name: app
      image: busybox:1.36
      command: ["sh", "-c", "echo 'hello from ipfs csi' > /data/hello.txt && sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: ipfs-pvc
```

## Read-only CID source

The driver also supports read-only mount scenarios where content is resolved from a fixed CID source.
Use a dedicated `StorageClass`/PVC workflow from your cluster policy when enabling this mode.

## Expand volume

Resize by increasing PVC request size:

```bash
kubectl patch pvc ipfs-pvc -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
```

The controller handles `ControllerExpandVolume`, and node-side expansion is processed during publish/expand operations.

## Snapshots

See [Snapshot and Restore](snapshot-and-restore.md) for complete flow and requirements.

## Common edge cases

- If mount paths already contain stale content, node publish/stage logic cleans and re-materializes content.
- Snapshot restore requires both snapshot CRDs and running `snapshot-controller`.
- Missing or mismatched snapshot class/driver name causes restore provisioning failures.

The E2E suites in `test/e2e/suites` are the canonical executable reference.
