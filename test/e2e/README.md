# E2E tests for csi-driver-ipfs

This test stack is fully rewritten to a Kubernetes-native layout:

- `Ginkgo v2 + Gomega` for suite-level behavior tests.
- `KUTTL` for declarative Kubernetes assertions.
- shared framework in `test/e2e/internal/framework` with centralized `BeforeSuite` and `AfterSuite` lifecycle logic.

Use `make -C test/e2e <target>` from the repo root.

## Test layout

- `test/e2e/suites/provisioning` - dynamic provisioning flows (single pod, two pods, reclaim delete).
- `test/e2e/suites/readonly` - read-only CID-mounted volume behavior.
- `test/e2e/suites/expand` - PVC expansion behavior.
- `test/e2e/suites/snapshot` - snapshot create and restore flows.
- `test/e2e/kuttl` - declarative control-plane assertions.

## Main targets

- `make -C test/e2e env/up` - create Kind cluster and deploy dependencies.
- `make -C test/e2e test/ginkgo` - run all Ginkgo suites.
- `make -C test/e2e test/kuttl` - run KUTTL assertions.
- `make -C test/e2e test/e2e` - run full E2E pipeline (Ginkgo + KUTTL).
- `make -C test/e2e env/down` - remove Kind cluster.

## Environment variables

| Variable             | Default                      | Description                                      |
|----------------------|------------------------------|--------------------------------------------------|
| `E2E_RUN`            | (unset)                      | Set to `1` or `true` to enable suite execution   |
| `E2E_STORAGE_CLASS`  | `ipfs-csi`                   | StorageClass used by dynamic provisioning suites |
| `E2E_NAMESPACE`      | `default`                    | Namespace used by all suites                     |
| `E2E_DRIVER_NAME`    | `ipfs.csi.ptrvsrg.github.io` | CSI driver name for snapshot/read-only classes   |
| `E2E_TIMEOUT`        | `180s`                       | Timeout used by suite setup and waits            |
| `E2E_POLL_INTERVAL`  | `2s`                         | Poll interval for wait loops                     |
| `E2E_RETRY_ATTEMPTS` | `2`                          | Retry attempts for create/wait operations        |
| `E2E_RETRY_DELAY`    | `3s`                         | Delay between operation retries                  |
| `KIND_CONTEXT`       | `kind-csi-ipfs-e2e`          | Context used by KUTTL and kubectl helpers        |
| `KUTTL_ENABLED`      | `1`                          | Set to `0` to skip KUTTL run in `test/e2e`       |
| `GINKGO_FOCUS`       | (unset)                      | Optional focus regexp for Ginkgo                 |

On retry failures, framework automatically dumps diagnostics:

- Pods/PVC/PV/snapshot objects
- Recent namespace events
- Tail logs from `csi-ipfs-controller` and `snapshot-controller` pods

## Example local flow

```bash
make -C test/e2e env/up
E2E_TIMEOUT=240s make -C test/e2e test/e2e
make -C test/e2e env/down
```
