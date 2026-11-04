package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	eraseA = "11111111-1111-4111-8111-111111111111"
	eraseB = "22222222-2222-4222-8222-222222222222"
)

func eraseReq(s *server, key, path string) int {
	req := httptest.NewRequest("DELETE", path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	w := httptest.NewRecorder()
	s.internalAPI().ServeHTTP(w, req)
	return w.Code
}

func TestEraseAccountRemovesOnlyThatAccount(t *testing.T) {
	t.Setenv("ACCOUNT_ERASE_KEY", "erase-key-0123456789")
	s := shopServerFor(t)
	core := testCore(s)
	core.addUser("aaaaaaaaaaaaaaaa", 1, "a")
	core.addUser("bbbbbbbbbbbbbbbb", 2, "b")
	core.rememberNA("na-a", "aaaaaaaaaaaaaaaa")
	core.rememberNA("na-b", "bbbbbbbbbbbbbbbb")
	core.accts[eraseA], core.accts[eraseB] = "na-a", "na-b"
	st := s.shop
	st.Entitlements = []*ShopEntitlement{{AccountID: "na-a", TitleID: "t1"}, {AccountID: "na-a", TitleID: "t2"}, {AccountID: "na-b", TitleID: "t1"}}
	st.Tasks = []*ShopTask{{ID: "x", AccountID: "na-a"}, {ID: "y", AccountID: "na-b"}}
	st.Links = deviceLinkTable{Links: []*beachDeviceLink{{DeviceID: "d1", AccountID: "na-a", BaasUserID: "aaaaaaaaaaaaaaaa"}, {DeviceID: "d1", AccountID: "na-b", BaasUserID: "bbbbbbbbbbbbbbbb"}}}
	st.Redemptions = []*BundleRedemption{{DeviceID: "d1", AccountID: "na-a"}, {DeviceID: "d1", AccountID: "na-b"}}
	st.Stats = map[string]*ShopTitleStats{"t1": {Downloads: 5}}
	st.save()

	p := "/internal/accounts/" + eraseA + "/data"
	if c := eraseReq(s, "", p); c != http.StatusUnauthorized {
		t.Fatalf("no key: %d", c)
	}
	if c := eraseReq(s, "wrong", p); c != http.StatusUnauthorized {
		t.Fatalf("wrong key: %d", c)
	}
	if c := eraseReq(s, "erase-key-0123456789", "/internal/accounts/nope/data"); c != 400 {
		t.Fatalf("bad id: %d", c)
	}
	if c := eraseReq(s, "erase-key-0123456789", p); c != 200 {
		t.Fatalf("erase: %d", c)
	}
	n, err := s.eraseAccount(t.Context(), eraseA)
	if err != nil || n != 0 {
		t.Fatalf("second run: %d %v", n, err)
	}
	if len(st.Entitlements) != 1 || st.Entitlements[0].AccountID != "na-b" || len(st.Tasks) != 1 || len(st.Links.Links) != 1 ||
		st.Links.Links[0].AccountID != "na-b" || len(st.Redemptions) != 1 || st.Redemptions[0].AccountID != "na-b" || st.Stats["t1"].Downloads != 5 {
		t.Fatalf("state after erase: %+v", st.doc())
	}
	// the Postgres document (here the memory store) lost them too, not only the live slices
	st2 := newShopStateWith(st.dir, st.store)
	if len(st2.Entitlements) != 1 || len(st2.Tasks) != 1 {
		t.Fatalf("persisted state: %+v", st2.doc())
	}
	// an account with no Switch identity has nothing here
	if n, err := s.eraseAccount(t.Context(), "33333333-3333-4333-8333-333333333333"); err != nil || n != 0 {
		t.Fatalf("unknown account: %d %v", n, err)
	}
	// identity core down: an error, nothing deleted, so the finaliser retries
	core.down = true
	if _, err := s.eraseAccount(t.Context(), eraseB); err == nil {
		t.Fatal("core down must fail")
	}
	core.down = false
	t.Setenv("ACCOUNT_ERASE_KEY", "")
	if c := eraseReq(s, "x", p); c != 503 {
		t.Fatalf("no env key: %d", c)
	}
}
