# Changelog -- nx-shop

## v0.4.1 -- 2026-11-06

- Card #148: the ledger holds no per-pack art (game cosmetics host none), so a pack shows its game's own icon (Don't Starve Together: the Switch icon, the same the website draws), fetched from `NX_SHOP_ICON_BASE` (default https://openpak.org) and cached; the pack's own art wins if the ledger ever has some. Gift icon only when neither exists.

## v0.4.0 -- 2026-11-06

- Game items for Rewards points: `GET /ashigaru/api/v1/game-items` and `POST .../game-items/redeem` relay through shop-core (`NX_SHOP_CORE_URL`, `NX_SHOP_CORE_KEY`) to the Rewards ledger by the player's BAAS id; a Game Items section in the storefront lists them (DST Wilson packs) and unlocks them for points. Points only. Each pack shows its own art (`GET /ashigaru/api/v1/game-items/{item}/art`, relayed through shop-core; the gift icon only when it has none); the "All titles are free" note is hidden on the Game Items screens. Unset env: the section is hidden and nothing changes.

## v0.3.1 -- 2026-11-05

- Card #141: pearljam `GET /civil/v1/consumables/<id>/items` answers an empty, well-formed list (`consumable_items: []`, `length`/`total` 0, `inc_tax_message`) instead of `{}`. Don't Starve Together asks for its currency-pack list (`BOLTPACKS0000001`) at the main menu and before its in-game Shop; without `consumable_items` it logs "Error parsing JSON while receiving product list" and the Shop stays on "Loading...". OpenPak sells nothing for money, so the list is empty. Kept pearljam request dumps no longer contain tokens, cookies or Authorization values.

## v0.3.0 -- 2026-11-04

- Account erasure (card #120): `DELETE /internal/accounts/{account_id}/data` with `Authorization: Bearer $ACCOUNT_ERASE_KEY` on the internal listener (`NX_API_ADDR`, default `:20070`, container network; console hosts never reach it). Unset key = 503, bad key = 401, non-UUID = 400. Reply `{"service":"nx-shop","rows":N}`, one `account erased` log line.
- The OpenPak account is mapped to its Nintendo Account id (and BAAS user id) through nx-baas's existing identity API (read-only, `NX_CORE_URL`). Erased: entitlements (the library and purchases), download tasks, beach device links, bundle redemptions. The state document is written before memory changes, so a failed write deletes nothing and the call can be repeated. An account with no Switch identity is 0 rows.
- Kept: per-title download statistics (aggregate counters, no account), the catalog, the firmware blocklist. No wishlist is stored here.

## v0.2.8 -- 2026-11-03

- Release workflow pushes the image with `GHCR_TOKEN` like the other nx-* repos (the repo token was denied `read_package`, so v0.2.6 and v0.2.7 never published). No code change from v0.2.7.

## v0.2.7 -- 2026-11-03

- The shop's player and admin state (entitlements, download tasks, device links, per-title stats, bundle redemptions, firmware blocklist) now lives in Postgres, table `shop_state` (one jsonb document, pool of 2), instead of `state.json` on the volume. `NX_SHOP_DATABASE_URL` is required; the service exits at start without it or if the database is unreachable.
- An existing `state.json` is imported once into an empty database, the counts are verified by reading the row back and logged, and the file is kept untouched. The catalog stays in `catalog.json` (content); `state.json`'s catalog mirror is no longer read.
- Tests: restart-in-Postgres and one-time-import tests (`NX_SHOP_TEST_DATABASE_URL`); the existing suite runs on an in-memory store.

## v0.2.6 -- 2026-11-03

- Every HTTP request now writes one access line to stderr: `http <method> <host> <path> -> <status> <ms>ms ua=...` (shared `nx-common/httplog`, vendored). The query string, headers and bodies are never logged; before, the request line carried the full URI including the query, and no status or duration. `/healthz` is skipped.
