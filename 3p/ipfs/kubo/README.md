# Patches

## `00-go-mod.patch`

Updates upstream `go.mod` from Go
`1.26.2` to `1.26.4`. This keeps Kubo's declared toolchain aligned with the
compiler used by the Docker build and fixes the Go stdlib CVEs reported
against binaries built with Go `1.26.2`: CVE-2026-33811, CVE-2026-33814,
CVE-2026-39820, CVE-2026-39823, CVE-2026-39825, CVE-2026-39826,
CVE-2026-39836, and CVE-2026-42499.
