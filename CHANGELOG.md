# Changelog -- nx-shop

## v0.2.6 -- 2026-11-03

- Every HTTP request now writes one access line to stderr: `http <method> <host> <path> -> <status> <ms>ms ua=...` (shared `nx-common/httplog`, vendored). The query string, headers and bodies are never logged; before, the request line carried the full URI including the query, and no status or duration. `/healthz` is skipped.
