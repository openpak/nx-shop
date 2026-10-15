package main

// Who is asking: the shop's only dependency on the identity core.
//
// In nx-baas these functions read baas.json and the ban cache directly. Here the tokens are
// still verified locally -- nx-baas signs every one of them with the BAAS key this service
// also holds -- and only the mappings come from the core's identity API
// (nx-baas /internal/identity, nxapi.User / nxapi.NA): which BAAS user an NSA id token's
// subject is and which Nintendo Account it carries, which BAAS user an NA id belongs to, and
// whether the account is locked. Pushmo's /p/<penne id> asks the penne API (nx-baas until
// Penne moves, nx-penne after) which users a console registration carries.
//
// A peer that cannot be reached is logged and treated as "unknown": the request goes on with
// what the token alone proves. In practice that means an NA id token still names its
// account (entitlements are keyed by it) while the BAAS user and the lock are unknown, so a
// core restart does not take the shop down with it. The lock is enforced again on the next
// request that reaches a live core.

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"slices"

	"github.com/openpak/nx-common/nxapi"
)

// identity is what the shop asks the core (nxapi.Client implements it; tests use a fake).
type identity interface {
	User(ctx context.Context, id string) (*nxapi.User, error)
	NA(ctx context.Context, naID string) (*nxapi.NA, error)
	Account(ctx context.Context, accountID string) (*nxapi.Account, error)
	Avatar(ctx context.Context, userID string) (*http.Response, error)
}

// registrations is what the shop asks the penne side.
type registrations interface {
	Registration(ctx context.Context, penneID string) (*nxapi.Registration, error)
	ConsoleRegistration(ctx context.Context, deviceCert string) (*nxapi.Registration, error)
	Broadcast(ctx context.Context, m nxapi.DeviceMessage) (*nxapi.DeviceResult, error)
}

// coreUser is the BAAS user, or nil when the core does not know it (or cannot be reached).
func (s *server) coreUser(ctx context.Context, id string) *nxapi.User {
	if s.core == nil || id == "" {
		return nil
	}
	u, err := s.core.User(ctx, id)
	if err != nil {
		if !errors.Is(err, nxapi.ErrNotFound) {
			log.Printf("[shop] identity: user %s: %v", id, err)
		}
		return nil
	}
	return u
}

// coreNA is the core's view of a Nintendo Account id, or nil when it cannot be reached.
func (s *server) coreNA(ctx context.Context, naID string) *nxapi.NA {
	if s.core == nil || naID == "" {
		return nil
	}
	n, err := s.core.NA(ctx, naID)
	if err != nil {
		log.Printf("[shop] identity: na %s: %v", naID, err)
		return nil
	}
	return n
}

// naLock is the account's ban state ("" when it may use the network).
func (s *server) naLock(ctx context.Context, naID string) string {
	if n := s.coreNA(ctx, naID); n != nil {
		return n.Lock
	}
	return ""
}

// shopPrincipal resolves the account acting on a shop request: a verified NA id token (or
// our own BAAS-issued NSA id token) in Authorization or Nintendo-Nsa-Id-Token, corroborated
// by the Nintendo-Account-Id header when present. A header id without a token is not trusted (principal binding).
func (s *server) shopPrincipal(r *http.Request) (accountID, baasUserID string) {
	accountID, baasUserID = s.shopPrincipalUnchecked(r)
	// A banned account is nobody to the store: no acquisition, no tasks, no ownership.
	if accountID != "" && s.naLock(r.Context(), accountID) != "" {
		log.Printf("[shop] principal: account %s is locked; refusing", accountID)
		return "", ""
	}
	return accountID, baasUserID
}

func (s *server) shopPrincipalUnchecked(r *http.Request) (accountID, baasUserID string) {
	headerID := r.Header.Get("Nintendo-Account-Id")
	for _, h := range []string{"Authorization", "Nintendo-Nsa-Id-Token"} {
		claims := s.bearerHeader(r, h)
		if claims == nil {
			continue
		}
		sub := str(claims, "sub")
		iss := str(claims, "iss")
		// The shop page may hold the network service account's id token instead
		// (systemWeb's setNsaIdTokenAvailableCallback): our BAAS signs it, its subject
		// is the BAAS user, and that user carries the Nintendo Account id.
		if iss == baasIssuer && str(claims, "typ") == "id_token" {
			if u := s.coreUser(r.Context(), sub); u != nil && u.NaID != "" {
				if headerID != "" && headerID != u.NaID {
					log.Printf("[shop] principal: header %s disagrees with nsa token's account %s; refusing", headerID, u.NaID)
					return "", ""
				}
				return u.NaID, u.ID
			}
			continue
		}
		// NA id tokens come from our accounts.nintendo.com surface; a token from anywhere
		// else does not name an account here.
		if sub == "" || iss != naIssuer {
			continue
		}
		if headerID != "" && headerID != sub {
			log.Printf("[shop] principal: header %s disagrees with token sub %s; refusing", headerID, sub)
			return "", ""
		}
		// Map the NA id to the core's BAAS user so entitlement rows carry both.
		if n := s.coreNA(r.Context(), sub); n != nil && n.BaasUserID != "" {
			return sub, n.BaasUserID
		}
		return sub, ""
	}
	return "", ""
}

// idTokenAccounts maps the id tokens NIM forwards to the Nintendo Accounts they prove:
// our NA id tokens name the account in sub; our BAAS id tokens name the BAAS user, whose
// account it is. A token that does not verify against our key proves nothing.
func (s *server) idTokenAccounts(tokens []string) []string {
	var out []string
	for _, t := range tokens {
		c, err := parseJWT(t, &s.key.PublicKey)
		if err != nil {
			continue
		}
		account := ""
		switch str(c, "iss") {
		case naIssuer:
			account = str(c, "sub")
		case baasIssuer:
			if u := s.coreUser(context.Background(), str(c, "sub")); u != nil {
				account = u.NaID
			}
		}
		if account != "" && !slices.Contains(out, account) {
			out = append(out, account)
		}
	}
	return out
}

// penneShopAccounts resolves /pushmo/p/<id>: the id is the console's penne registration
// (hardware 2026-10-04: NIM's download-list task asked for /p/<the id beach and dragons
// were handed in penne_id/register>, never /d/). The registration's BAAS users map to
// the Nintendo Accounts the store keys tasks by.
func (s *server) penneShopAccounts(penneID string) []string {
	if s.penne == nil {
		return nil
	}
	ctx := context.Background()
	reg, err := s.penne.Registration(ctx, penneID)
	if err != nil {
		log.Printf("[shop] pushmo: penne registration %s: %v", penneID, err)
		return nil
	}
	return s.registrationAccounts(ctx, reg)
}

// consoleAccounts is every profile's Nintendo Account on the console a device certificate
// names, through its Penne registration. Licence checks (dragons rights) carry only that
// certificate; without this a profile with no device link was told it owns nothing and its
// install hung on a blank HOME tile (hardware 2026-10-05, 2206-1017 on launch).
func (s *server) consoleAccounts(deviceCert string) []string {
	if s.penne == nil || deviceCert == "" {
		return nil
	}
	ctx := context.Background()
	reg, err := s.penne.ConsoleRegistration(ctx, deviceCert)
	if err != nil {
		log.Printf("[shop] rights: console %s: %v", deviceCert, err)
		return nil
	}
	return s.registrationAccounts(ctx, reg)
}

func (s *server) registrationAccounts(ctx context.Context, reg *nxapi.Registration) []string {
	if reg == nil || !reg.Live {
		return nil
	}
	var out []string
	for _, uid := range append([]string{reg.UserID}, reg.Users...) {
		if u := s.coreUser(ctx, uid); u != nil && u.NaID != "" && !slices.Contains(out, u.NaID) {
			out = append(out, u.NaID)
		}
	}
	return out
}

// serveAvatar relays the user's picture from the core: the same one the console's profile
// shows, served on the shop's own origin.
func (s *server) serveAvatar(w http.ResponseWriter, r *http.Request, baasUser string) {
	if s.core == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	resp, err := s.core.Avatar(r.Context(), baasUser)
	if err != nil {
		log.Printf("[shop] avatar for %s: %v", baasUser, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for _, h := range []string{"Content-Type", "Content-Length", "Cache-Control", "ETag", "Last-Modified"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, 4<<20))
}
