# nx-shop

The OpenPak native eShop for a real Switch on firmware 22.5.0 and 23.0.1, split out of
[nx-baas](../nx-baas) so a shop release restarts only the shop: consoles keep their push
connections and logins, and nothing but the store notices a deploy.

It serves the whole store chain a console walks, unchanged from nx-baas:
savanna catalog/acquire/tasks -> requestSyncRights (dragons store rights routes, forwarded by
nx-baas) -> aqua -> pushmo `/p/` -> superfly `dv` -> veer -> atum HEAD/GET CNMT with
`x-nintendo-*` headers -> dragonst `issuable_titles`/`issue` with `na_id_tokens` -> atum c2
program with the bare DenebEdgeToken -> install. Plus beach (device links, digital bundles),
tigers, tagaya, idbe, bvc, pearljam and the store admin API the website's admin pages use.

Store design and contracts: [`docs/native-shop.md`](docs/native-shop.md),
[`docs/store-admin-api.md`](docs/store-admin-api.md). How the split works, the internal API
and the cut-over/rollback runbook: nx-baas `docs/service-split.md`.

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

## Who it asks

Tokens are verified here with the BAAS key. The mappings come from nx-baas's identity API
(`ident.go`): which BAAS user an NSA id token names and its Nintendo Account, which BAAS user
an NA id belongs to, whether the account is locked, an OpenPak account's Switch id (admin
account views), the user's picture. Pushmo `/p/<penne id>` asks the penne API which users a
console registration carries. With nx-baas unreachable an NA id token still names its account,
so the shop keeps working through a core restart.

## Building

`nx-common` (TLS, JWT, internal key, the internal API client) is a sibling module with no
remote yet: `go.mod` replaces it with `../nx-common` and `vendor/` holds the copy the image
build uses, so the build needs only this repository. After changing nx-common, run
`go mod vendor` here and commit `vendor/`.

    go vet ./... && go test ./...

Releases are tags: `v*.*.*` runs `.github/workflows/release.yml` on the self-hosted podman
runner and publishes `ghcr.io/openpak/nx-shop`.

## Porting a fix from nx-baas

The shop files (`shop.go`, `shop_data.go`, `store_admin.go`, `atum.go`, `beach.go`,
`eshop_*.go`) are nx-baas's at commit `f2cb368`, and the helpers they call keep their nx-baas
names (`common.go`), so a fix made there applies here as a patch until nx-baas drops its copy:

    git -C ../nx-baas diff f2cb368 origin/main -- shop.go shop_data.go store_admin.go atum.go \
        beach.go eshop_nim.go eshop_unserved.go eshop_bundle.go eshop_idbe.go | git apply -3

The only deliberate differences: `shopPrincipal`, `shopPrincipalUnchecked`, `idTokenAccounts`
and `penneShopAccounts` moved to `ident.go`; `/me/avatar` and the admin account lookup ask
the core. A hunk touching those needs porting by hand.
