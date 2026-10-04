package main

// What is new in the shop as its own service: routing only its own hostnames, the
// forwarded requests nx-baas hands over, and asking the identity core over HTTP instead of
// reading baas.json.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openpak/nx-common/nxapi"
)

// A hostname the shop does not own is a 421 and a log line, never another family's answer.
func TestForeignHostsAreMisrouted(t *testing.T) {
	s := shopServerFor(t)
	for _, c := range []struct{ host, path string }{
		{"m-lp1.baas.nintendo.com", "/1.0.0/login"},
		{"accounts.nintendo.com", "/connect/1.0.0/authorize"},
		{"god.hac.lp1.penne.srv.nintendo.net", "/v1/penne_ids"},
		{"api-lp1.pctl.srv.nintendo.net", "/moon/v1/devices"},
		// dragons. keeps its licence answers in nx-baas; only the store rights routes are ours.
		{"dragons.hac.lp1.dragons.nintendo.net", "/v1/elicenses/exercise"},
	} {
		if rec := hostReq(s, c.host, "POST", c.path, "", ""); rec.Code != http.StatusMisdirectedRequest {
			t.Errorf("%s%s = %d, want 421", c.host, c.path, rec.Code)
		}
	}
	if rec := hostReq(s, "dragons.hac.lp1.dragons.nintendo.net", "POST", "/v1/rights/publish_elicenses", "", `{"rights_ids":[]}`); rec.Code != 200 {
		t.Errorf("dragons store rights route = %d", rec.Code)
	}
	if rec := hostReq(s, "anything", "GET", "/healthz", "", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "nx-shop") {
		t.Errorf("healthz = %d %s", rec.Code, rec.Body)
	}
}

// nx-baas forwards console requests to the internal listener with the console's Host kept;
// they must get the same answers there as on the TLS listener, and the admin API lives there.
func TestInternalListenerServesForwardedConsoleRoutes(t *testing.T) {
	t.Setenv("NX_INTERNAL_KEY", "k")
	s := shopServerFor(t)
	h := s.internalAPI()
	for _, c := range []struct{ host, path, want string }{
		{"hac.lp1.savanna.srv.nintendo.net", "/top", "OpenPak Shop"},
		{"superfly.hac.lp1.d4c.nintendo.net", "/v1/a/" + testTitleID + "/dv", `"title_type":"Application"`},
		{"gw.hac.lp1.vermillion.srv.nintendo.net", "/v1/a/" + testTitleID + "/dv", `"title_type":"Application"`},
		{"c-lp1.accounts.nintendo.com", "/v1/apps/" + testTitleID + "0000000000000000/catalog.json", `"catalog_id"`},
		{"unknown.nintendo.net", "/v1/acds/search?q=openpak", `"acds"`},
	} {
		req := httptest.NewRequest("GET", "http://openpak-nx-shop:20070"+c.path, nil)
		req.Host = c.host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), c.want) {
			t.Errorf("forwarded %s%s = %d %.200s", c.host, c.path, rec.Code, rec.Body)
		}
	}
	req := httptest.NewRequest("GET", "http://openpak-nx-shop:20070/internal/store/titles", nil)
	req.Header.Set("X-Internal-Key", "k")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("store admin on the internal listener = %d", rec.Code)
	}
}

// The store treats a locked account as nobody; the lock comes from the core.
func TestLockedAccountIsNobodyToTheStore(t *testing.T) {
	s := shopServerFor(t)
	testCore(s).locks[testNAID] = "banned"
	rec := shopReq(s, "POST", "/ashigaru/api/v1/catalog/"+testTitleID+"/acquire", naIDTokenFor(s), "")
	if rec.Code != 401 {
		t.Fatalf("locked acquire = %d %s, want 401", rec.Code, rec.Body)
	}
}

// With the core unreachable an NA id token still names its account -- entitlements are keyed
// by it -- so the shop keeps working through a core restart; an NSA id token, which only the
// core can map to an account, identifies nobody until it is back.
func TestShopKeepsWorkingWhileTheCoreIsDown(t *testing.T) {
	s := shopServerFor(t)
	u := testCore(s).userByPID(7)
	testCore(s).setNaID(u.ID, testNAID)
	testCore(s).down = true
	rec := shopReq(s, "POST", "/ashigaru/api/v1/catalog/"+testTitleID+"/acquire", naIDTokenFor(s), "")
	if rec.Code != 200 {
		t.Fatalf("acquire with the core down = %d %s", rec.Code, rec.Body)
	}
	if e := s.shop.entitlementFor(testNAID, testTitleID); e == nil || e.BaasUserID != "" {
		t.Fatalf("entitlement = %+v; the BAAS user is unknown while the core is down", e)
	}
	rec = shopReq(s, "GET", "/ashigaru/api/v1/ownership", nsaIDTokenFor(s, u.ID), "")
	if rec.Code != 401 {
		t.Fatalf("nsa token with the core down = %d, want 401", rec.Code)
	}
	testCore(s).down = false
	rec = shopReq(s, "GET", "/ashigaru/api/v1/ownership", nsaIDTokenFor(s, u.ID), "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), testTitleID) {
		t.Fatalf("nsa token with the core back = %d %s", rec.Code, rec.Body)
	}
}

// The real client against a stand-in for nx-baas's identity and penne routes: the JSON
// field names on the wire are the ones nx-baas's internal_identity_test.go pins.
func TestShopTalksToTheCoreOverHTTP(t *testing.T) {
	t.Setenv("NX_INTERNAL_KEY", "k")
	const userID = "0123456789abcdef"
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Key") != "k" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/identity/na/" + testNAID:
			w.Write([]byte(`{"na_id":"` + testNAID + `","baas_user_id":"` + userID + `","pid":7,"lock":""}`))
		case "/internal/identity/users/" + userID:
			w.Write([]byte(`{"id":"` + userID + `","na_id":"` + testNAID + `","pid":7,"nickname":"tester","lock":""}`))
		case "/internal/identity/users/" + userID + "/avatar":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(testJPEG)
		case "/internal/penne/registrations/pz1":
			json.NewEncoder(w).Encode(map[string]any{"penne_id": "pz1", "live": true, "user_id": userID, "users": []string{}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer core.Close()
	s := shopServerFor(t)
	c := nxapi.New(core.URL)
	s.core, s.penne = c, c

	tok := naIDTokenFor(s)
	if rec := shopReq(s, "POST", "/ashigaru/api/v1/catalog/"+testTitleID+"/acquire", tok, ""); rec.Code != 200 {
		t.Fatalf("acquire = %d %s", rec.Code, rec.Body)
	}
	if e := s.shop.entitlementFor(testNAID, testTitleID); e == nil || e.BaasUserID != userID {
		t.Fatalf("entitlement = %+v, want the core's BAAS user on it", e)
	}
	if rec := shopReq(s, "GET", "/ashigaru/api/v1/me/avatar", tok, ""); rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("avatar = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := shopReq(s, "POST", "/ashigaru/api/v1/tasks", tok, `{"title_id":"`+testTitleID+`"}`); rec.Code != 200 {
		t.Fatalf("task = %d", rec.Code)
	}
	rec := hostReq(s, "pushmo.hac.lp1.eshop.nintendo.net", "GET", "/pushmo/p/pz1.json", "", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"owner_application":"`+testTitleID+`"`) {
		t.Fatalf("pushmo p over HTTP = %d %s", rec.Code, rec.Body)
	}
}
