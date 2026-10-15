package main

// Test stand-ins for the services the shop talks to, and the tokens nx-baas would have
// minted. The tests themselves are nx-baas's shop tests, kept as close to the originals as
// the seam allows: where they used baas.json directly they now use fakeCore, which answers
// what nx-baas's /internal/identity answers for the same state.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/openpak/nx-common/nxapi"
)

func dauthServer(t *testing.T) *server {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return &server{key: key, idKid: "test-kid"}
}

// fakeCore is nx-baas's identity API over an in-memory user table.
type fakeCore struct {
	mu    sync.Mutex
	users map[string]*nxapi.User
	na    map[string]string // NA id -> BAAS user id (nx-baas: NaToPID, then UserByPID)
	locks map[string]string // NA id -> lock
	accts map[string]string // core account id -> NA id
	down  bool              // every call fails, as with nx-baas restarting
	calls int
}

func newFakeCore() *fakeCore {
	return &fakeCore{users: map[string]*nxapi.User{}, na: map[string]string{}, locks: map[string]string{}, accts: map[string]string{}}
}

func testCore(s *server) *fakeCore { return s.core.(*fakeCore) }

func (c *fakeCore) addUser(id string, pid uint64, nickname string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users[id] = &nxapi.User{ID: id, PID: pid, Nickname: nickname}
}

func (c *fakeCore) rememberNA(naID, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.na[naID] = userID
}

func (c *fakeCore) setNaID(userID, naID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users[userID].NaID = naID
}

func (c *fakeCore) userByPID(pid uint64) *nxapi.User {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, u := range c.users {
		if u.PID == pid {
			cp := *u
			return &cp
		}
	}
	return nil
}

var errCoreDown = fmt.Errorf("fake core: connection refused")

func (c *fakeCore) User(_ context.Context, id string) (*nxapi.User, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.down {
		return nil, errCoreDown
	}
	u := c.users[id]
	if u == nil {
		return nil, nxapi.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (c *fakeCore) NA(_ context.Context, naID string) (*nxapi.NA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.down {
		return nil, errCoreDown
	}
	out := &nxapi.NA{NaID: naID, Lock: c.locks[naID]}
	if u := c.users[c.na[naID]]; u != nil {
		out.BaasUserID, out.PID = u.ID, u.PID
	}
	return out, nil
}

func (c *fakeCore) Account(_ context.Context, accountID string) (*nxapi.Account, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.down {
		return nil, errCoreDown
	}
	if na := c.accts[accountID]; na != "" {
		return &nxapi.Account{AccountID: accountID, NaID: na}, nil
	}
	return nil, nxapi.ErrNotFound
}

// testJPEG is the smallest thing the avatar route's content check would call a JPEG.
var testJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0}

func (c *fakeCore) Avatar(_ context.Context, userID string) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.down {
		return nil, errCoreDown
	}
	if c.users[userID] == nil {
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}
	h := http.Header{"Content-Type": {"image/jpeg"}, "Cache-Control": {"public, max-age=3600"}, "Etag": {`"pic-1"`}}
	return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(bytes.NewReader(testJPEG))}, nil
}

// fakePenne is the penne API's registrations.
type fakePenne struct {
	mu       sync.Mutex
	regs     map[string]*nxapi.Registration
	consoles map[string]string // device certificate -> penne id
	sent     []nxapi.DeviceMessage
}

func newFakePenne() *fakePenne {
	return &fakePenne{regs: map[string]*nxapi.Registration{}, consoles: map[string]string{}}
}

func (p *fakePenne) Broadcast(_ context.Context, m nxapi.DeviceMessage) (*nxapi.DeviceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent = append(p.sent, m)
	return &nxapi.DeviceResult{Connections: 1}, nil
}

func (p *fakePenne) ConsoleRegistration(ctx context.Context, cert string) (*nxapi.Registration, error) {
	p.mu.Lock()
	id := p.consoles[cert]
	p.mu.Unlock()
	return p.Registration(ctx, id)
}

func testPenne(s *server) *fakePenne { return s.penne.(*fakePenne) }

func (p *fakePenne) register(userID string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := "pz" + randHex(15)
	p.regs[id] = &nxapi.Registration{PenneID: id, Live: true, UserID: userID, Users: []string{}}
	return id
}

func (p *fakePenne) Registration(_ context.Context, id string) (*nxapi.Registration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r := p.regs[id]; r != nil {
		return r, nil
	}
	return &nxapi.Registration{PenneID: id, Users: []string{}}, nil
}

// ---------- the tokens nx-baas mints, minted the same way ----------

// naIDTokenForNA is nx-baas's naIDToken for an NA id (accounts.nintendo.com).
func naIDTokenForNA(s *server, naID string) string {
	now := time.Now().Unix()
	return signJWT(map[string]any{"alg": "RS256", "jku": naIssuer + "/1.0.0/certificates", "kid": "na-kid", "typ": "JWT"},
		map[string]any{"aud": "test-client", "exp": now + 900, "iat": now, "iss": naIssuer, "jti": uuid4(), "sub": naID}, s.key)
}

// nsaIDTokenFor is nx-baas's BAAS id_token (the network service account's) for a user.
func nsaIDTokenFor(s *server, userID string) string {
	now := time.Now().Unix()
	return signJWT(map[string]any{"alg": "RS256", "kid": s.idKid, "typ": "JWT"},
		map[string]any{"aud": "ed9e2f05d286f7b8", "bs:did": userID, "exp": now + 10800, "iat": now, "iss": baasIssuer, "jti": uuid4(), "sub": userID, "typ": "id_token"}, s.key)
}

// deviceTokenFor is nx-baas's dauth device token.
func deviceTokenFor(s *server, device string) string {
	now := time.Now().Unix()
	return signJWT(map[string]any{"alg": "RS256", "kid": s.idKid, "typ": "JWT"},
		map[string]any{"iss": "dauth-lp1.ndas.srv.nintendo.net", "jti": uuid4(), "iat": now - 600, "exp": now + 86400, "sub": device, "aud": "test"}, s.key)
}

// edgeDToken is nx-baas's dauth edge token format (dauth.go).
func edgeDToken(exp int64, device string) string {
	return fmt.Sprintf("exp=%d~acl=%%2F%%2A~data=sub=%s.sn=%s.pc=HAC.id=%s~hmac=%s",
		exp, device, "XKJ00000000000", uuid4(), randHex(32))
}
