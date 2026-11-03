package main

// shop_pg.go: Postgres is the store of the shop's player and admin state (owner rule
// 2026-11-03: state lives in Postgres, not in files on a volume). The state -- entitlements,
// download tasks, device links, per-title stats, bundle redemptions and the firmware
// blocklist -- is one document, saved whole under the state mutex exactly where state.json
// used to be written. The catalog is content (catalog.json, icons, screens, packages) and
// stays in files.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const shopSchema = `
CREATE TABLE IF NOT EXISTS shop_state (
	id         int PRIMARY KEY CHECK (id = 1),
	doc        jsonb NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now()
)`

// shopDoc is the persisted state document.
type shopDoc struct {
	Entitlements    []*ShopEntitlement         `json:"entitlements"`
	Tasks           []*ShopTask                `json:"tasks"`
	Links           deviceLinkTable            `json:"links"`
	Stats           map[string]*ShopTitleStats `json:"stats,omitempty"`
	Redemptions     []*BundleRedemption        `json:"bundle_redemptions,omitempty"`
	BlockedFirmware []BlockedFirmware          `json:"blocked_firmware,omitempty"`
}

func (d *shopDoc) counts() string {
	return fmt.Sprintf("%d entitlements, %d tasks, %d device links, %d stats, %d bundle redemptions, %d blocked firmware",
		len(d.Entitlements), len(d.Tasks), len(d.Links.Links), len(d.Stats), len(d.Redemptions), len(d.BlockedFirmware))
}

// shopStore keeps the state document: Postgres in production, memory in tests.
type shopStore interface {
	loadDoc() (doc []byte, found bool, err error)
	saveDoc(doc []byte) error
}

type memShopStore struct {
	mu  sync.Mutex
	doc []byte
}

func (m *memShopStore) loadDoc() ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doc, m.doc != nil, nil
}

func (m *memShopStore) saveDoc(b []byte) error {
	m.mu.Lock()
	m.doc = b
	m.mu.Unlock()
	return nil
}

type pgShopStore struct{ pool *pgxpool.Pool }

// connectShopPG opens a small pool and creates the table. The error never carries the DSN.
func connectShopPG(ctx context.Context, dsn string) (*pgShopStore, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("NX_SHOP_DATABASE_URL is not a valid Postgres URL")
	}
	cfg.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, shopSchema); err != nil {
		pool.Close()
		return nil, err
	}
	return &pgShopStore{pool: pool}, nil
}

func (p *pgShopStore) loadDoc() ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var raw []byte
	err := p.pool.QueryRow(ctx, `SELECT doc FROM shop_state WHERE id = 1`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return raw, err == nil, err
}

func (p *pgShopStore) saveDoc(b []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := p.pool.Exec(ctx, `INSERT INTO shop_state (id, doc) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET doc = EXCLUDED.doc, updated_at = now()`, b)
	return err
}

// openShopState is production startup: Postgres is required (fatal otherwise). When the
// database holds no state yet, the old <dir>/state.json is imported once, the counts are
// verified by reading the row back, and the file is left in place.
func openShopState(dir, dsn string) *shopState {
	if dsn == "" {
		log.Fatalf("%s NX_SHOP_DATABASE_URL is required (Postgres is the shop's state store)", logPrefix)
	}
	pg, err := connectShopPG(context.Background(), dsn)
	if err != nil {
		log.Fatalf("%s postgres: %v", logPrefix, err)
	}
	s, err := newShopStateFrom(dir, pg)
	if err != nil {
		log.Fatalf("%s postgres state: %v", logPrefix, err)
	}
	s.mu.Lock()
	log.Printf("%s shop state in Postgres: %s", logPrefix, s.doc().counts())
	s.mu.Unlock()
	return s
}

// newShopStateFrom loads the state from store, importing dir/state.json first when the
// store is empty.
func newShopStateFrom(dir string, store shopStore) (*shopState, error) {
	s := &shopState{dir: dir, store: store, Catalog: map[string]*ShopTitle{}}
	raw, found, err := store.loadDoc()
	if err != nil {
		return nil, err
	}
	if found {
		if err := s.applyDoc(raw); err != nil {
			return nil, err
		}
	} else if dir != "" {
		if b, rerr := os.ReadFile(dir + "/state.json"); rerr == nil {
			if err := s.applyDoc(b); err != nil {
				return nil, fmt.Errorf("state.json: %w", err)
			}
			want := s.doc().counts()
			if err := store.saveDoc(s.docBytes()); err != nil {
				return nil, err
			}
			back, ok, err := store.loadDoc()
			if err != nil || !ok {
				return nil, fmt.Errorf("verify import: %v", err)
			}
			chk := &shopState{}
			if err := chk.applyDoc(back); err != nil {
				return nil, err
			}
			if got := chk.doc().counts(); got != want {
				return nil, fmt.Errorf("import verification failed: file %s, database %s", want, got)
			}
			log.Printf("%s imported %s/state.json (%d bytes) into Postgres: %s; the file is kept", logPrefix, dir, len(b), want)
		}
	}
	s.loadCatalogFile()
	if len(s.Catalog) == 0 {
		s.Catalog = defaultShopCatalog()
	}
	return s, nil
}

func (s *shopState) applyDoc(b []byte) error {
	var d shopDoc
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	s.Entitlements, s.Tasks, s.Links, s.Stats = d.Entitlements, d.Tasks, d.Links, d.Stats
	s.Redemptions, s.BlockedFirmware = d.Redemptions, d.BlockedFirmware
	return nil
}

func (s *shopState) doc() *shopDoc {
	return &shopDoc{Entitlements: s.Entitlements, Tasks: s.Tasks, Links: s.Links, Stats: s.Stats,
		Redemptions: s.Redemptions, BlockedFirmware: s.BlockedFirmware}
}

func (s *shopState) docBytes() []byte {
	b, _ := json.Marshal(s.doc())
	return b
}
