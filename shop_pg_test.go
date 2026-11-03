package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// testShopDSN is a scratch database (NX_SHOP_TEST_DATABASE_URL); its state row is emptied
// first. Unset skips, unless NX_SHOP_TEST_REQUIRE_DB is set.
func testShopDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("NX_SHOP_TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("NX_SHOP_TEST_REQUIRE_DB") != "" {
			t.Fatal("NX_SHOP_TEST_DATABASE_URL is required")
		}
		t.Skip("NX_SHOP_TEST_DATABASE_URL not set")
	}
	pg, err := connectShopPG(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.pool.Close()
	if _, err := pg.pool.Exec(context.Background(), `DELETE FROM shop_state`); err != nil {
		t.Fatal(err)
	}
	return dsn
}

func openTestShop(t *testing.T, dir, dsn string) *shopState {
	t.Helper()
	pg, err := connectShopPG(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.pool.Close)
	s, err := newShopStateFrom(dir, pg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Restart test: entitlements, tasks, links, stats and the blocklist survive a new instance
// on the same database, with the same elicense id.
func TestShopStateSurvivesRestartInPostgres(t *testing.T) {
	dsn := testShopDSN(t)
	dir := t.TempDir()
	a := openTestShop(t, dir, dsn)
	e, _, err := a.acquire(testNAID, "u1", testTitleID, testDevice, 65536)
	if err != nil {
		t.Fatal(err)
	}
	task, err := a.createTask(testNAID, testTitleID)
	if err != nil {
		t.Fatal(err)
	}
	a.linkDevice(testDevice, testNAID, "u1")
	a.mu.Lock()
	a.Stats = map[string]*ShopTitleStats{testTitleID: {Downloads: 3, BytesServed: 99}}
	a.BlockedFirmware = []BlockedFirmware{{Version: "22-5-0", AddedAt: 1}}
	a.save()
	a.mu.Unlock()

	b := openTestShop(t, dir, dsn)
	if got := b.entitlementFor(testNAID, testTitleID); got == nil || got.ELicenseID != e.ELicenseID {
		t.Fatalf("entitlement lost across restart: %+v", got)
	}
	if b.task(task.ID, testNAID) == nil {
		t.Error("task lost across restart")
	}
	if len(b.Links.Links) != 1 || b.Stats[testTitleID].Downloads != 3 || len(b.BlockedFirmware) != 1 {
		t.Errorf("links/stats/blocklist lost: %s", b.doc().counts())
	}
}

// state.json is imported once into an empty database, verified, and kept.
func TestLegacyStateJSONImportedOnceAndKept(t *testing.T) {
	dsn := testShopDSN(t)
	dir := t.TempDir()
	old := newShopState("")
	old.acquire(testNAID, "u1", testTitleID, testDevice, 1)
	old.createTask(testNAID, testTitleID)
	legacy := old.docBytes()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	a := openTestShop(t, dir, dsn)
	if len(a.Entitlements) != 1 || len(a.Tasks) != 1 {
		t.Fatalf("import: %s", a.doc().counts())
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Error("state.json was not kept")
	}
	a.revoke(testNAID, testTitleID)
	// Second start must keep the DB state (revoked), not re-import the file.
	b := openTestShop(t, dir, dsn)
	if e := b.entitlementFor(testNAID, testTitleID); e == nil || e.Status != "revoked" {
		t.Errorf("second start re-imported the file: %+v", e)
	}
}
