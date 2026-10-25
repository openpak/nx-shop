# Protocol: nx-shop

It serves the whole store chain a console walks, unchanged from nx-baas:
savanna catalog/acquire/tasks -> requestSyncRights (dragons store rights routes, forwarded by
nx-baas) -> aqua -> pushmo `/p/` -> superfly `dv` -> veer -> atum HEAD/GET CNMT with
`x-nintendo-*` headers -> dragonst `issuable_titles`/`issue` with `na_id_tokens` -> atum c2
program with the bare DenebEdgeToken -> install. Plus beach (device links, digital bundles),
tigers, tagaya, idbe, bvc, pearljam and the store admin API the website's admin pages use.

Tokens are verified here with the BAAS key. The mappings come from nx-baas's identity API
(`ident.go`): which BAAS user an NSA id token names and its Nintendo Account, which BAAS user
an NA id belongs to, whether the account is locked, an OpenPak account's Switch id (admin
account views), the user's picture. Pushmo `/p/<penne id>` asks the penne API which users a
console registration carries. With nx-baas unreachable an NA id token still names its account,
so the shop keeps working through a core restart.
