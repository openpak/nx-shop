# Notes: nx-shop

The OpenPak native eShop for a real Switch on firmware 22.5.0 and 23.0.1, split out of
[nx-baas](../../nx-baas) so a shop release restarts only the shop: consoles keep their push
connections and logins, and nothing but the store notices a deploy.

Store design and contracts: [`native-shop.md`](native-shop.md),
[`store-admin-api.md`](store-admin-api.md). How the split works, the internal API
and the cut-over/rollback runbook: nx-baas `docs/service-split.md`.

## Porting a fix from nx-baas

The shop files (`shop.go`, `shop_data.go`, `store_admin.go`, `atum.go`, `beach.go`,
`eshop_*.go`) are nx-baas's at commit `f2cb368`, and the helpers they call keep their nx-baas
names (`common.go`), so a fix made there applies here as a patch until nx-baas drops its copy:

    git -C ../nx-baas diff f2cb368 origin/main -- shop.go shop_data.go store_admin.go atum.go \
        beach.go eshop_nim.go eshop_unserved.go eshop_bundle.go eshop_idbe.go | git apply -3

The only deliberate differences: `shopPrincipal`, `shopPrincipalUnchecked`, `idTokenAccounts`
and `penneShopAccounts` moved to `ident.go`; `/me/avatar` and the admin account lookup ask
the core. A hunk touching those needs porting by hand.
