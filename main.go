// nx-shop: the OpenPak native eShop for a real Switch, split out of nx-baas so a shop release
// restarts only the shop (nx-baas docs/service-split.md).
//
// It owns the store's hostnames end to end -- the shop applet pages and their API (savanna,
// bugyo), the content host (atum and the rest of d4c), the rights family's store side
// (beach, dragonst, tigers, and the store rights routes of dragons, which nx-baas forwards
// here), NIM's download and version lists (pushmo, tagaya, superfly, veer, aqua, sun), the
// digital bundles, idbe, bvc and pearljam -- and the store's state under $NX_SHOP_DIR (the
// catalog, entitlements, tasks, device links and the uploaded packages). What it needs to
// know about people it asks the identity core (ident.go).
//
// Two listeners:
//
//	NX_ADDR      TLS for consoles (Traefik passes the SNIs through), certificates from the
//	             OpenPak CA in NX_CA_DIR, exactly as nx-baas terminates them (nx-common/nxtls)
//	NX_API_ADDR  plain HTTP on the openpak network: the store admin API (/internal/store, the
//	             website's), and the console routes again for what nx-baas forwards here
package main

import (
	"bufio"
	"bytes"
	"crypto/rsa"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/openpak/nx-common/nxapi"
	"github.com/openpak/nx-common/nxhttp"
	"github.com/openpak/nx-common/nxjwt"
	"github.com/openpak/nx-common/nxtls"
	"time"
)

type server struct {
	// shop is the store's state (catalog, entitlements, tasks, beach links); shopUI holds
	// the storefront's in-memory diagnostics ring.
	shop   *shopState
	shopUI *shopUI
	// key is the BAAS signing key: it verifies the tokens nx-baas mints (NA and NSA id
	// tokens, device tokens) and signs the dragons edge tokens atum checks.
	key   *rsa.PrivateKey
	idKid string
	// core answers who a user or account is (nx-baas); penne which users a console's
	// registration carries (nx-baas until Penne moves, then nx-penne).
	core  identity
	penne registrations
}

// fixedNames are the shop names that were on nx-baas's fixed certificate, kept fixed here:
// the console authenticates some names strictly and refused an on-demand certificate on
// those (2123-0013). The rest of the family (savanna, pushmo, tagaya, veer, superfly, aqua,
// sun, idbe, bvc) got on-demand certificates there and gets them here, as does 23.x's pegasus.
var fixedNames = []string{
	"bugyo.hac.lp1.eshop.nintendo.net", "beach.hac.lp1.eshop.nintendo.net", "pearljam.hac.lp1.eshop.nintendo.net",
	"atum.hac.lp1.d4c.nintendo.net", "tigers.hac.lp1.dragons.nintendo.net", "dragonst.hac.lp1.dragons.nintendo.net",
}

var debug = os.Getenv("NX_DEBUG") != ""

// ServeHTTP routes by host, in nx-baas's order for these families. A host outside them is
// misrouted -- a stale Traefik router or forward -- and says so with a 421.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := hostOf(r)
	log.Printf("%s %s %s%s ua=%q", logPrefix, r.Method, host, r.URL.RequestURI(), r.UserAgent())
	// The friends module fetches its app catalog and art by path on its own host; nx-baas
	// forwards those here, and this declines anything that is not /v1/apps/<id>/.
	if s.friendsAppCatalog(w, r) {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if debug && !strings.HasPrefix(r.URL.Path, "/ashigaru/icon/") {
		dump(r, body, "DBG >>>")
		rec := &recorder{ResponseWriter: w, status: 200}
		w = rec
		defer func() {
			log.Printf("%s DBG <<< %d %s %s%s body(%d)=%s", logPrefix, rec.status, r.Method, host, r.URL.Path, rec.buf.Len(), rec.buf.Bytes())
		}()
	}
	switch {
	case r.URL.Path == "/healthz":
		writeJSON(w, 200, map[string]any{"status": "ok", "service": "nx-shop"})
	case strings.HasPrefix(host, "pearljam."):
		// The eShop host NIM names beside dragons and tigers; /civil and /sugar
		// answer a checked, recorded success (eshop_unserved.go).
		s.pearljam(w, r, body)
	case strings.HasPrefix(host, "idbe-hac."):
		// NIM's icon data: Nintendo's 3DS / Wii U files relayed (eshop_idbe.go).
		s.idbe(w, r, body)
	case strings.HasPrefix(host, "bvc-hac-"):
		// NIM's firmware-version check against the operator blocklist (eshop_unserved.go).
		s.bvc(w, r)
	case strings.HasPrefix(host, "bugyo."):
		s.shopHandler(w, r, body)
	case strings.HasSuffix(host, ".savanna.srv.nintendo.net"):
		// Where 22.5.0's systemWeb opens the shop (shop.go).
		s.savanna(w, r, body)
	case strings.HasPrefix(host, "pushmo."):
		s.pushmo(w, r, body)
	case strings.HasPrefix(host, "tagaya."):
		s.tagaya(w, r, body)
	case strings.HasPrefix(host, "pegasus."):
		// 23.x's version list and recommended versions (tagaya and superfly before it).
		s.pegasus(w, r, body)
	case strings.HasPrefix(host, "veer."):
		s.veer(w, r, body)
	// NIM's other d4c hosts: system-update metadata (sun, aqua), the version lookup
	// (superfly) and atumn, claimed before the content routes below.
	case isNimMetaHost(host):
		if !s.nimUnserved(w, r, body) {
			dump(r, body, "!! UNHANDLED nim metadata host")
			writeJSON(w, 404, map[string]any{"status": 404, "title": "not found"})
		}
	case strings.Contains(host, ".d4c.nintendo.net"):
		s.atum(w, r, body)
	case strings.HasPrefix(host, "dragonst.") || strings.HasPrefix(host, "tigers."):
		s.dragons(w, r, body)
	case strings.HasPrefix(host, "dragons.") && shopRightsRoute(r.URL.Path):
		// Only the store rights routes: the rest of dragons. stays with nx-baas, which
		// keeps the licence answers every title needs and forwards these.
		s.dragons(w, r, body)
	case strings.HasPrefix(host, "beach."):
		s.beach(w, r, body)
	// The catalog routes nx-baas forwards from hosts it keeps (the vermillion gateway and
	// its catch-all): superfly's version lookup and the catalogue search.
	case s.nimUnserved(w, r, body):
	default:
		nxhttp.Misrouted(logPrefix, w, r)
	}
}

// internalAPI is the plain-HTTP listener: the console routes for requests nx-baas forwards
// with the console's Host kept, and -- for every other Host -- the store admin API and a
// health check. A console host never reaches the admin routes (nxhttp.SplitListener).
func (s *server) internalAPI() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/store/", s.storeAdmin)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "service": "nx-shop"})
	})
	return nxhttp.SplitListener(s, mux)
}

func main() {
	key, err := nxjwt.LoadRSAKey(getenv("BAAS_SIGNING_KEY", "/keys/baas_signing_key.pem"))
	if err != nil {
		log.Fatalf("%s signing key: %v", logPrefix, err)
	}
	coreURL := os.Getenv("NX_CORE_URL")
	if coreURL == "" {
		log.Fatalf("%s NX_CORE_URL is required (nx-baas's internal listener, e.g. http://openpak-nx-baas:20070)", logPrefix)
	}
	core := nxapi.New(coreURL)
	// Penne's API is served by nx-baas until Penne has its own service.
	penne := core
	if u := os.Getenv("NX_PENNE_URL"); u != "" {
		penne = nxapi.New(u)
	}
	shopDir := getenv("NX_SHOP_DIR", getenv("NX_DATA", "/data")+"/shop")
	s := &server{
		shop:  newShopState(shopDir),
		key:   key,
		idKid: getenv("BAAS_KID", "openpak-baas-key-1"),
		core:  core,
		penne: penne,
	}
	go func() {
		addr := getenv("NX_API_ADDR", ":20070")
		log.Printf("%s internal API listening %s (store admin, forwarded console routes)", logPrefix, addr)
		log.Fatal(http.ListenAndServe(addr, s.internalAPI()))
	}()
	go s.watchVersionList(30 * time.Second)
	srv := &http.Server{
		Addr: getenv("NX_ADDR", ":21000"), Handler: s,
		TLSConfig: nxtls.ServerConfig(logPrefix, fixedNames, getenv("NX_CA_DIR", "/data/ca"), debug),
	}
	log.Printf("%s listening %s; shop data %s; core %s; titles=%d", logPrefix, srv.Addr, shopDir, coreURL, len(s.shop.Catalog))
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

// ---------- debug recorder (nx-baas main.go) ----------

type recorder struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (rec *recorder) WriteHeader(c int) { rec.status = c; rec.ResponseWriter.WriteHeader(c) }
func (rec *recorder) Write(b []byte) (int, error) {
	if rec.buf.Len() < 8192 {
		rec.buf.Write(b)
	}
	return rec.ResponseWriter.Write(b)
}

func (rec *recorder) Flush() {
	if f, ok := rec.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rec *recorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := rec.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("recorder: wrapped writer is not a Hijacker")
}
