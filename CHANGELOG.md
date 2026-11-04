# Changelog -- nx-shop

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
