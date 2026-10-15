package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A reserved id is served its control NCA from a file, outside the catalog, with the headers NIM
// takes the content's identity from; an id with no file, and an ordinary id that merely looks
// close, are not served by this path.
func TestAtumServesReservedControlOutsideTheCatalog(t *testing.T) {
	s, _ := contentServerFor(t)
	const id = "01fe00ff00006000"
	nca := []byte("not a real NCA, just bytes that hash")
	dir := filepath.Join(s.shop.dir, "reserved")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".nca"), nca, 0o600); err != nil {
		t.Fatal(err)
	}
	host := "atum.hac.lp1.d4c.nintendo.net"
	live := edgeDToken(timeNowUnix()+3600, testDevice)

	rec := atumReq(s, "GET", host, "/c1/a/d/"+id, live)
	sum := sha256Hex(nca)
	h := rec.Header()
	if rec.Code != 200 || rec.Body.String() != string(nca) || h.Get("X-Nintendo-Content-Hash") != sum ||
		h.Get("X-Nintendo-Content-Id") != sum[:32] || h.Get("X-Nintendo-Title-Version") != "0" {
		t.Fatalf("reserved control = %d, headers %v", rec.Code, h)
	}
	if rec := atumReq(s, "GET", host, "/c1/a/d/"+id, ""); rec.Code != 403 {
		t.Errorf("no token = %d, want 403 (reserved ids are not public)", rec.Code)
	}
	if rec := atumReq(s, "GET", host, "/c1/a/d/01fe00ff00007000", live); rec.Code != 404 {
		t.Errorf("reserved id with no file = %d, want 404", rec.Code)
	}
	if rec := atumReq(s, "GET", host, "/c1/a/d/01fe00fe00006000", live); rec.Code != 404 {
		t.Errorf("an id outside the reserved prefix = %d, want 404", rec.Code)
	}
	// It is not a catalog title: nothing lists it.
	if s.shop.Catalog[id] != nil {
		t.Error("a reserved title must not be in the catalog")
	}
}
