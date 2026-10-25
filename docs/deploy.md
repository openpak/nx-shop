# Deploy: nx-shop

## Running

| env | default | |
|---|---|---|
| `NX_CORE_URL` | (required) | nx-baas's internal listener, e.g. `http://openpak-nx-baas:20070` |
| `NX_PENNE_URL` | `NX_CORE_URL` | who serves `/internal/penne` (nx-penne once Penne has moved) |
| `NX_INTERNAL_KEY` | | the shared internal key; the store admin API refuses everyone without it |
| `NX_ADDR` | `:21000` | TLS for consoles (Traefik SNI passthrough) |
| `NX_API_ADDR` | `:20070` | plain HTTP: `/internal/store/*` and the console routes nx-baas forwards |
| `NX_SHOP_DIR` | `$NX_DATA/shop` | catalog.json, state.json, icons, screens, uploaded packages |
| `NX_DATA` | `/data` | scratch beside it: `idbe/` icon cache, `pearljam/` request dumps |
| `NX_CA_DIR` | `/data/ca` | `ca.pem`/`ca.key` that sign the listener's certificates |
| `BAAS_SIGNING_KEY`, `BAAS_KID` | `/keys/baas_signing_key.pem`, `openpak-baas-key-1` | nx-baas's key: verifies its tokens, signs edge tokens |
| `NX_SHOP_UPLOAD_MAX_BYTES` | 8 GiB | store upload limit |
| `NX_DEBUG` | | log every request and reply, and every TLS hello |

A hostname this service does not own is answered 421 and logged (a stale router).
`deploy/` has a sample quadlet and the Traefik router; neither is applied anywhere.

## Building

`nx-common` (TLS, JWT, internal key, the internal API client) is a sibling module with no
remote yet: `go.mod` replaces it with `../nx-common` and `vendor/` holds the copy the image
build uses, so the build needs only this repository. After changing nx-common, run
`go mod vendor` here and commit `vendor/`.

    go vet ./... && go test ./...

Releases are tags: `v*.*.*` runs `.github/workflows/release.yml` on the self-hosted podman
runner and publishes `ghcr.io/openpak/nx-shop`.
