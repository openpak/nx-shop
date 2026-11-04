package main

// Account erasure (card #120): nx-shop keeps entitlements (the library), download tasks, beach
// device links and bundle redemptions, all keyed by the Switch Nintendo Account id (and the BAAS
// user id on links). The OpenPak account id is mapped to those through nx-baas's identity API,
// which still knows the account while the website finaliser runs.

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/openpak/nx-common/nxapi"
)

var eraseUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// eraseAccount removes every shop row of the OpenPak account and returns how many. Idempotent. An
// account with no Switch identity has nothing here. The state document is written before memory is
// changed, so a failed write leaves everything as it was and the finaliser retries.
func (s *server) eraseAccount(ctx context.Context, accountID string) (int, error) {
	if s.core == nil {
		return 0, errors.New("no identity core")
	}
	acct, err := s.core.Account(ctx, accountID)
	if errors.Is(err, nxapi.ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	naID, baasID := acct.NaID, ""
	if na, err := s.core.NA(ctx, naID); err != nil {
		return 0, err
	} else {
		baasID = na.BaasUserID
	}
	mine := func(na, baas string) bool {
		return (naID != "" && strings.EqualFold(na, naID)) || (baasID != "" && baas != "" && strings.EqualFold(baas, baasID))
	}

	st := s.shop
	st.mu.Lock()
	defer st.mu.Unlock()
	var ents []*ShopEntitlement
	var tasks []*ShopTask
	var links []*beachDeviceLink
	var reds []*BundleRedemption
	for _, e := range st.Entitlements {
		if !mine(e.AccountID, e.BaasUserID) {
			ents = append(ents, e)
		}
	}
	for _, t := range st.Tasks {
		if !mine(t.AccountID, "") {
			tasks = append(tasks, t)
		}
	}
	for _, l := range st.Links.Links {
		if !mine(l.AccountID, l.BaasUserID) {
			links = append(links, l)
		}
	}
	for _, r := range st.Redemptions {
		if !mine(r.AccountID, "") {
			reds = append(reds, r)
		}
	}
	rows := len(st.Entitlements) - len(ents) + len(st.Tasks) - len(tasks) + len(st.Links.Links) - len(links) + len(st.Redemptions) - len(reds)
	if rows > 0 {
		d := st.doc()
		d.Entitlements, d.Tasks, d.Links, d.Redemptions = ents, tasks, deviceLinkTable{Links: links}, reds
		if st.store != nil {
			b, _ := json.Marshal(d)
			if err := st.store.saveDoc(b); err != nil {
				return 0, err
			}
		}
		st.Entitlements, st.Tasks, st.Links, st.Redemptions = ents, tasks, deviceLinkTable{Links: links}, reds
	}
	log.Printf("account erased account=%s rows=%d service=nx-shop", strings.ToLower(accountID), rows)
	return rows, nil
}

// eraseHandler serves DELETE /internal/accounts/{account_id}/data on the internal listener.
func (s *server) eraseHandler(w http.ResponseWriter, r *http.Request) {
	key := os.Getenv("ACCOUNT_ERASE_KEY")
	if key == "" {
		writeJSON(w, 503, map[string]any{"error": "erase disabled"})
		return
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(key)) != 1 {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("account_id")
	if !eraseUUID.MatchString(id) {
		writeJSON(w, 400, map[string]any{"error": "bad account id"})
		return
	}
	n, err := s.eraseAccount(r.Context(), strings.ToLower(id))
	if err != nil {
		log.Printf("[shop] erase failed: %v", err)
		writeJSON(w, 502, map[string]any{"error": "erase failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"service": "nx-shop", "rows": n})
}
