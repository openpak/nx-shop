# Changelog -- nx-shop

## v0.2.8 -- 2026-11-03

- Release workflow pushes the image with `GHCR_TOKEN` like the other nx-* repos (the repo token was denied `read_package`, so v0.2.6 and v0.2.7 never published). No code change from v0.2.7.

## v0.2.7 -- 2026-11-03

- The shop's player and admin state (entitlements, download tasks, device links, per-title stats, bundle redemptions, firmware blocklist) now lives in Postgres, table `shop_state` (one jsonb document, pool of 2), instead of `state.json` on the volume. `NX_SHOP_DATABASE_URL` is required; the service exits at start without it or if the database is unreachable.
- An existing `state.json` is imported once into an empty database, the counts are verified by reading the row back and logged, and the file is kept untouched. The catalog stays in `catalog.json` (content); `state.json`'s catalog mirror is no longer read.
- Tests: restart-in-Postgres and one-time-import tests (`NX_SHOP_TEST_DATABASE_URL`); the existing suite runs on an in-memory store.

## v0.2.6 -- 2026-11-03

- Every HTTP request now writes one access line to stderr: `http <method> <host> <path> -> <status> <ms>ms ua=...` (shared `nx-common/httplog`, vendored). The query string, headers and bodies are never logged; before, the request line carried the full URI including the query, and no status or duration. `/healthz` is skipped.
