package main

// Game items for Rewards points (card #148). The shop lists a game's cosmetics (for example the Don't
// Starve Together Wilson packs) next to its homebrew and unlocks them for Rewards points. It is an
// adapter: shop-core relays to the one Rewards ledger, so this shop, the website and the games' own
// shops spend the same points. Points only, never money. Off (no shop-core configured) the list is
// empty and the shop shows nothing new.

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type shopCoreClient struct {
	base, key string
	hc        *http.Client
}

func newShopCoreClient(base, key string) *shopCoreClient {
	if base == "" || key == "" {
		return nil
	}
	return &shopCoreClient{base: strings.TrimRight(base, "/"), key: key, hc: &http.Client{Timeout: 10 * time.Second}}
}

func (c *shopCoreClient) do(r *http.Request, method, path string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, c.base+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Internal-Key", c.key)
	req.Header.Set("X-Shop-Actor", "nx-shop")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, b, err
}

// shopGameItems answers GET /ashigaru/api/v1/game-items: {enabled, balance, linked, items}. Not configured:
// {enabled:false, items:[]}, so the page hides the section. An unlinked or signed-out player sees the items with 0 points.
func (s *server) shopGameItems(w http.ResponseWriter, r *http.Request, baasUser string) {
	if s.shopCore == nil {
		writeJSON(w, 200, map[string]any{"enabled": false, "items": []any{}})
		return
	}
	subject := baasUser
	if subject == "" {
		subject = "-"
	}
	code, b, err := s.shopCore.do(r, "GET", "/v1/shop/switch/game-items?subject="+url.QueryEscape(subject), nil)
	if err != nil || code != 200 {
		log.Printf("%s game items: shop-core status %d err %v", logPrefix, code, err)
		writeJSON(w, 200, map[string]any{"enabled": false, "items": []any{}})
		return
	}
	var in struct {
		Account string           `json:"account"`
		Balance int64            `json:"balance"`
		Items   []map[string]any `json:"items"`
	}
	if json.Unmarshal(b, &in) != nil {
		writeJSON(w, 200, map[string]any{"enabled": false, "items": []any{}})
		return
	}
	if in.Items == nil {
		in.Items = []map[string]any{}
	}
	// A pack has no art of its own unless the ledger holds some; the game's own icon stands in for it.
	for _, it := range in.Items {
		if g, _ := it["game"].(string); gameIconID[g] != "" {
			it["art"] = true
		}
	}
	writeJSON(w, 200, map[string]any{"enabled": true, "linked": in.Account != "", "balance": in.Balance, "items": in.Items})
}

// shopGameRedeem answers POST /ashigaru/api/v1/game-items/redeem {game, game_ref, expected_price, trans_id}. The
// ledger's answer and status pass through: 200 {balance}, 402 insufficient, 409 owned | price_mismatch | not_for_sale,
// 404 not linked to an OpenPak account.
func (s *server) shopGameRedeem(w http.ResponseWriter, r *http.Request, baasUser string, body []byte) {
	if s.shopCore == nil {
		writeJSON(w, 503, map[string]any{"error": map[string]any{"code": "3001", "message": "game items are not available"}})
		return
	}
	if baasUser == "" {
		writeJSON(w, 401, map[string]any{"error": map[string]any{"code": "1002", "message": "account identity required"}})
		return
	}
	var in struct {
		Game          string `json:"game"`
		GameRef       string `json:"game_ref"`
		ExpectedPrice *int   `json:"expected_price"`
		TransID       string `json:"trans_id"`
	}
	if json.Unmarshal(body, &in) != nil || in.Game == "" || in.GameRef == "" {
		writeJSON(w, 400, map[string]any{"error": map[string]any{"code": "1001", "message": "game and game_ref are required"}})
		return
	}
	out := map[string]any{"subject": baasUser, "game": in.Game, "game_ref": in.GameRef, "trans_id": in.TransID}
	if in.ExpectedPrice != nil {
		out["expected_price"] = *in.ExpectedPrice
	}
	code, b, err := s.shopCore.do(r, "POST", "/v1/shop/switch/game-items/redeem", out)
	if err != nil {
		log.Printf("%s game redeem: shop-core err %v", logPrefix, err)
		writeJSON(w, 502, map[string]any{"error": map[string]any{"code": "3002", "message": "the points ledger did not answer"}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(b)
}

// gameIconID is each hosted game's Switch title id: its icon is the art a pack shows when it has none of its own
// (the website draws the same icon, /titles/icon/<id>).
var gameIconID = map[string]string{"dst": "010090100E334000"}

var gameIcons sync.Map // game -> iconBlob

type iconBlob struct {
	ct   string
	data []byte
}

// shopGameItemArt answers GET /ashigaru/api/v1/game-items/{item}/art?game=: the pack's own picture from the ledger (through
// shop-core) when it has one, else the game's icon. Public like the catalog icons; 404 when there is neither.
func (s *server) shopGameItemArt(w http.ResponseWriter, r *http.Request, item string) {
	if s.shopCore == nil || item == "" || strings.ContainsAny(item, "/?#% ") {
		http.NotFound(w, r)
		return
	}
	ct, data := "", []byte(nil)
	req, err := http.NewRequestWithContext(r.Context(), "GET", s.shopCore.base+"/v1/shop/switch/game-items/"+url.PathEscape(item)+"/art", nil)
	if err == nil {
		req.Header.Set("X-Internal-Key", s.shopCore.key)
		if resp, err := s.shopCore.hc.Do(req); err == nil {
			if resp.StatusCode == 200 {
				ct = resp.Header.Get("Content-Type")
				data, _ = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			}
			resp.Body.Close()
		}
	}
	if len(data) == 0 {
		g := r.URL.Query().Get("game")
		id := gameIconID[g]
		if id == "" {
			http.NotFound(w, r)
			return
		}
		if v, ok := gameIcons.Load(g); ok {
			b := v.(iconBlob)
			ct, data = b.ct, b.data
		} else {
			base := getenv("NX_SHOP_ICON_BASE", "https://openpak.org")
			rq, _ := http.NewRequestWithContext(r.Context(), "GET", strings.TrimRight(base, "/")+"/titles/icon/"+id, nil)
			resp, err := s.shopCore.hc.Do(rq)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			data, _ = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			ct = resp.Header.Get("Content-Type")
			ok := resp.StatusCode == 200 && strings.HasPrefix(ct, "image/")
			resp.Body.Close()
			if !ok || len(data) == 0 {
				http.NotFound(w, r)
				return
			}
			gameIcons.Store(g, iconBlob{ct, data})
		}
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(data)
}
