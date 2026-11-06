package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGameItemsOffByDefault(t *testing.T) {
	s := shopServerFor(t)
	m := decodeBody(t, shopReq(s, "GET", "/ashigaru/api/v1/game-items", naIDTokenFor(s), ""))
	if m["enabled"] != false {
		t.Fatalf("expected disabled, got %v", m)
	}
	if rec := shopReq(s, "POST", "/ashigaru/api/v1/game-items/redeem", naIDTokenFor(s), `{"game":"dst","game_ref":"x"}`); rec.Code != 503 {
		t.Fatalf("redeem off: %d", rec.Code)
	}
}

func TestGameItemsListAndRedeemThroughShopCore(t *testing.T) {
	var path, key, body string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, key, body = r.URL.RequestURI(), r.Header.Get("X-Internal-Key"), string(b)
		if r.Method == "POST" {
			w.WriteHeader(402)
			w.Write([]byte(`{"code":"insufficient","balance":5}`))
			return
		}
		w.Write([]byte(`{"account":"acc","balance":120,"items":[{"game":"dst","game_ref":"pack_x","slot":"pack","name":"X Chest","price":80}]}`))
	}))
	defer core.Close()
	s := shopServerFor(t)
	s.shopCore = newShopCoreClient(core.URL, "k")
	tok := naIDTokenFor(s)
	m := decodeBody(t, shopReq(s, "GET", "/ashigaru/api/v1/game-items", tok, ""))
	if m["enabled"] != true || m["balance"] != float64(120) || m["linked"] != true || len(m["items"].([]any)) != 1 {
		t.Fatalf("list: %v", m)
	}
	if !strings.HasPrefix(path, "/v1/shop/switch/game-items?subject=0123456789abcdef") || key != "k" {
		t.Fatalf("relay: %s %s", path, key)
	}
	rec := shopReq(s, "POST", "/ashigaru/api/v1/game-items/redeem", tok, `{"game":"dst","game_ref":"pack_x","expected_price":80,"trans_id":"t1"}`)
	if rec.Code != 402 || !strings.Contains(body, `"subject":"0123456789abcdef"`) || !strings.Contains(body, `"expected_price":80`) {
		t.Fatalf("redeem: %d %s", rec.Code, body)
	}
	if rec := shopReq(s, "POST", "/ashigaru/api/v1/game-items/redeem", "", `{"game":"dst","game_ref":"pack_x"}`); rec.Code != 401 {
		t.Fatalf("anonymous redeem: %d", rec.Code)
	}
}
