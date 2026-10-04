package main

// The helpers the shop files were written against in nx-baas, under the same names, so a fix
// made to one of those files there applies here as a patch (README.md, "Porting a fix from nx-baas"). The
// bodies live in nx-common.

import (
	"net/http"
	"strings"

	"github.com/openpak/nx-common/nxhttp"
	"github.com/openpak/nx-common/nxjwt"
)

// The issuers of the tokens a shop request may carry: nx-baas mints both with the one BAAS
// signing key this service also holds.
const (
	baasIssuer = "https://e0d67c509fb203858ebcb2fe3f88c2aa.baas.nintendo.com"
	naIssuer   = "https://accounts.nintendo.com"
)

const logPrefix = "[nx-shop]"

func getenv(k, d string) string                               { return nxhttp.Getenv(k, d) }
func writeJSON(w http.ResponseWriter, status int, v any)      { nxhttp.WriteJSON(w, status, v) }
func writeJSONValue(w http.ResponseWriter, status int, v any) { nxhttp.WriteJSONValue(w, status, v) }
func hostOf(r *http.Request) string                           { return nxhttp.HostOf(r) }
func dump(r *http.Request, body []byte, tag string)           { nxhttp.Dump(logPrefix, r, body, tag) }
func str(m map[string]any, k string) string                   { return nxjwt.Str(m, k) }
func randHex(n int) string                                    { return nxjwt.RandHex(n) }
func uuid4() string                                           { return nxjwt.UUID4() }

var (
	parseJWT = nxjwt.Parse
	signJWT  = nxjwt.Sign
)

// deviceIDFromUA pulls the console's own device id out of the SDK's User-Agent:
// "NintendoSDK Firmware/<ver> (platform:NX; did:<hex>; eid:lp1)". (nx-baas penne.go)
func deviceIDFromUA(ua string) string {
	i := strings.Index(ua, "did:")
	if i < 0 {
		return ""
	}
	rest := ua[i+len("did:"):]
	if j := strings.IndexAny(rest, "; )"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
