# Switch eShop Server

OpenPak's Switch eShop Server.

| | |
| --- | --- |
| Network | Nintendo Switch (firmware 22.5.0 and 23.0.1) |
| Role | Native eShop for real Switch consoles: store catalog, acquire, install chain and store admin API |
| Depends on | [`nx-baas`](https://github.com/openpak/nx-baas) (identity, tokens, internal listener), `nx-common` (vendored in `vendor/`) |
| Used by | [`nx-baas`](https://github.com/openpak/nx-baas) forwards the console store routes here |
| Image | `ghcr.io/openpak/nx-shop` |
| Licence | AGPL-3.0-only |

## What it serves

| Endpoint | What |
| --- | --- |
| `:21000` (TLS, `NX_ADDR`) | Store chain consoles walk: catalog, acquire, rights, CNMT, issue and install |
| `:20070` (HTTP, `NX_API_ADDR`) | `/internal/store/*` and the console routes nx-baas forwards |
| Beach, tigers, tagaya, idbe, bvc, pearljam | Device links, digital bundles and other store services |

## Run

```sh
NX_CORE_URL=http://openpak-nx-baas:20070 go run .
```

## Docs

- [Protocol](docs/protocol.md)
- [Evidence](docs/evidence.md)
- [Deploy](docs/deploy.md)
- [Notes](docs/notes.md)
- [Store design](docs/native-shop.md)
- [Store admin API](docs/store-admin-api.md)

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
