package main

// The NIM hosts that carry downloads and updates rather than rights: pushmo (the download
// list), tagaya (the version list), superfly (one title's versions), pegasus (both of
// those on 23.x), veer (which content vendor to use) and the metadata hosts that share
// atum's d4c domain.
//
// OpenPak's store carries homebrew and the updates for it. A title has an application
// content meta and, once an update is published, a patch under the patch id; everything
// here answers from the catalog in shop_data.go, which is re-read when the operator edits
// catalog.json (reloadCatalog). So publishing an update is: place the patch's files, raise
// the title's patch version in catalog.json, and the next version-list poll carries it.
//
// What is known about each body, and from where [firmware, 22.5.0]:
//
//   - superfly /v1/a/<application id>/dv: a JSON array of {title_id, title_type, version}.
//     NIM's reply handler (vtable 0x3280e0, element callback 0x20f090) reads title_id as a
//     base-16 string, title_type as one of Application, Patch, AddOnContent, DataPatch, and
//     version as an unsigned 32-bit integer. Verified.
//   - tagaya /tagaya/hac_versionlist: ns parses $.last_modified and $.titles[].{id,version,
//     required_version} (ns 0x1f66d0, 0x1f6950), id through a base-16 conversion. NIM sends
//     If-None-Match with the ETag it kept. Field set verified; whether id is the
//     application or the patch id is not — the application id is served, because the
//     version list is ns's per-application record (IVersionListImporter).
//   - pushmo /pushmo/{d,p}/<id>.json: NIM fetches them with If-None-Match and keeps the
//     ETag; nothing in NIM parses the body. The report's candidate ($[].title_id...) turned
//     out to be the superfly dv handler above. The best remaining candidate is ns's download
//     task list parser (ns 0x22a090, 0x22a674): $.tasks[].{id (a 36-character UUID),
//     owner_application, na_id, config, titles[].{id, version, type}}, type being
//     Application, Patch, AddOnContent, Delta or DataPatch. Both answer in that shape from
//     the open download tasks: /d/<device> through device links, /p/<penne id> through the
//     penne registration's accounts. Hardware (22.5.0) asks /p/. Every request is logged.
//   - veer /v5/policy.json?context=: $.ttl and $.weights[].{vendor, weight} (NIM
//     0x2277e0, 0x228230). vendor is a CDN name looked up in NIM's vendor table (akamai,
//     lumen, llnw, fastly, cloudflare), each of which names the atum subdomain and edge
//     token header NIM then uses; akamai is atum with X-Nintendo-DenebEdgeToken. Any
//     other name fails the whole policy (0xab489) and NIM keeps its default, which is
//     also akamai: that is why serving "atum" here looked verified on 22.5.0.
//
// What 23.0.1 changed [firmware 23.0.1, nim 0100000000000025, ns 010000000000001f]:
//
//   - tagaya is gone. The version list is GET pegasus.hac.<env>.pegasus.srv.nintendo.net
//     /v1/versionlist (nim URL builder 0x1f2570, task nn.nim.VersionListTask), still with
//     If-None-Match, and ns parses it as {"formatVersion", "lastModified",
//     "cnmts":[{"id", "version", "requiredVersion"}]} (ns 0x1f8c64..0x1f8f74):
//     formatVersion must be 2 when present (else ns 0x14410), id is a base-16 string,
//     version and requiredVersion are integers, and an entry without id or version fails
//     the import (0x3c10). A missing requiredVersion is taken to be the version itself
//     -- a forced update -- so it is always sent.
//   - superfly's dv is gone. Its replacement is GET pegasus /v1/apps/<application id>
//     /recommend-versions (0x1f1ff0), parsed at 0x214490..0x214750 as
//     {"recommendVersions":[{"cnmtId" (base-16 string), "cnmtType" (Application, Patch,
//     AddOnContent, DataPatch), "recommendVersion" (u32)}]}. A 404 is "no record"
//     (0x19089, tolerated by the caller 0x214880), as superfly's was.
//   - pegasus also serves /v1/ratinglist (0x1f2640, nn.nim.RatingListTask); its body is
//     not recovered and is refused here. olsc reaches pegasus for its own requests.
//   - veer is /v6/policy.json (0x1f2920; parser 0x230460). Same fields; the vendor table
//     is now akamai, fastly, cloudflare, cloudfront (atum, atum-5a8, atum-f03, atum-b6f),
//     at most six weights, each <= 10000.
//   - aqua's /required_system_update_meta is no longer fetched at all.
//   - Every ETag-carrying task (pushmo, versionlist, ratinglist, atumn, idbe) keeps the
//     ETag only when it is quoted, with an optional W/ (0x21e300), and sends it back as
//     If-None-Match: [W/]"<tag>" (0x21df50). writeETagged's strong tag round-trips.
//   - pegasus requests carry an edge token from dauth client 0xe95fddf58447f117 (new in
//     23.x; nx-baas's dauth issues for any client id). Nothing here checks it.

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
)

// isNimMetaHost: the d4c hosts that are not content. sun and aqua answer system-update
// questions, superfly the version lookup, atumn* is NIM's other atum family. They route to
// nimUnserved's path table instead of atum.
func isNimMetaHost(host string) bool {
	if !strings.Contains(host, ".d4c.nintendo.net") {
		return false
	}
	for _, p := range []string{"sun.", "superfly.", "aqua.", "atumn"} {
		if strings.HasPrefix(host, p) {
			return true
		}
	}
	return false
}

// writeETagged answers a document with a strong ETag over its bytes, and 304 when the
// console already holds that version. The tag changes exactly when the body does.
func writeETagged(w http.ResponseWriter, r *http.Request, body []byte) {
	sum := sha1.Sum(body)
	etag := `"` + hex.EncodeToString(sum[:10]) + `"`
	w.Header().Set("ETag", etag)
	if m := r.Header.Get("If-None-Match"); m != "" && (m == "*" || strings.Contains(m, etag)) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

// ---------- superfly ----------

// versionLookup answers /v1/a/<application id>/dv from the catalog: the application's
// own content meta and, when one is published, its patch. A title this store does not
// carry has no version record here, which is what 404 says.
func (s *server) versionLookup(w http.ResponseWriter, r *http.Request, body []byte) {
	s.ensureShop()
	s.shop.reloadCatalog()
	id := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/a/"), "/dv"))
	recs := s.versionRecords(id)
	if recs == nil {
		log.Printf("[shop] superfly dv %s -> not in the catalog", id)
		writeJSON(w, 404, map[string]any{"status": 404, "title": "no version record"})
		return
	}
	out := make([]map[string]any, 0, len(recs))
	for _, v := range recs {
		out = append(out, map[string]any{"title_id": v.id, "title_type": v.typ, "version": v.version})
	}
	log.Printf("[shop] superfly dv %s -> %d version record(s)", id, len(out))
	writeJSONValue(w, 200, out)
}

type versionRecord struct {
	id, typ string
	version uint32
}

// versionRecords is one application's version records from the catalog: its own content
// meta and, when one is published, its patch. nil when the store does not carry it.
func (s *server) versionRecords(id string) []versionRecord {
	t, _ := s.shop.meta(id)
	if t == nil {
		return nil
	}
	s.shop.mu.Lock()
	defer s.shop.mu.Unlock()
	out := []versionRecord{{t.TitleID, "Application", t.applicationVersion()}}
	if t.Patch != nil {
		out = append(out, versionRecord{patchIDFor(t.TitleID), "Patch", t.Patch.Version})
	}
	return out
}

func (s *server) ensureShop() {
	if s.shop == nil {
		s.shop = newShopState("")
	}
}

// ---------- tagaya ----------

// tagaya serves 22.x's version list (23.x asks pegasus instead; see versionList).
func (s *server) tagaya(w http.ResponseWriter, r *http.Request, body []byte) {
	s.ensureShop()
	if r.URL.Path != "/tagaya/hac_versionlist" {
		dump(r, body, "!! UNHANDLED tagaya")
		writeJSON(w, 404, map[string]any{"status": 404, "title": "not found"})
		return
	}
	doc, n := s.versionList(false)
	log.Printf("[shop] tagaya version list: %d title(s), if-none-match=%q", n, r.Header.Get("If-None-Match"))
	writeETagged(w, r, []byte(mustJSON(doc)))
}

// versionList renders the version list: the newest version of every catalog title, keyed by
// application id. lastModified is the newest catalog change rather than the clock, so the
// body -- and with it the ETag -- changes only when a version is published. v2 is the
// 23.x document (pegasus /v1/versionlist), else 22.x's tagaya one. requiredVersion is
// always 0: OpenPak never forces an update, and 23.x reads a missing one as the version.
func (s *server) versionList(v2 bool) (map[string]any, int) {
	s.shop.reloadCatalog()
	entries := []map[string]any{}
	var modified int64
	for _, t := range s.shopTitles() {
		e := map[string]any{"id": t.TitleID, "version": t.Version, "required_version": 0}
		if v2 {
			e = map[string]any{"id": t.TitleID, "version": t.Version, "requiredVersion": 0}
		}
		entries = append(entries, e)
		modified = max(modified, t.UpdatedAt)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i]["id"].(string) < entries[j]["id"].(string) })
	if v2 {
		return map[string]any{"formatVersion": 2, "lastModified": modified, "cnmts": entries}, len(entries)
	}
	return map[string]any{"format_version": 1, "last_modified": modified, "titles": entries}, len(entries)
}

// ---------- pegasus (23.x) ----------

// pegasus answers the 23.x update hosts: the version list and the per-application
// recommended versions that replaced tagaya and superfly. Anything else on the host
// (ratinglist, olsc's requests) is recorded and refused.
func (s *server) pegasus(w http.ResponseWriter, r *http.Request, body []byte) {
	s.ensureShop()
	p := r.URL.Path
	switch {
	case p == "/v1/versionlist":
		doc, n := s.versionList(true)
		log.Printf("[shop] pegasus version list: %d title(s), if-none-match=%q", n, r.Header.Get("If-None-Match"))
		writeETagged(w, r, []byte(mustJSON(doc)))
	case strings.HasPrefix(p, "/v1/apps/") && strings.HasSuffix(p, "/recommend-versions"):
		id := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(p, "/v1/apps/"), "/recommend-versions"))
		s.shop.reloadCatalog()
		recs := s.versionRecords(id)
		if recs == nil {
			log.Printf("[shop] pegasus recommend-versions %s -> not in the catalog", id)
			writeJSON(w, 404, map[string]any{"status": 404, "title": "no version record"})
			return
		}
		out := make([]map[string]any, 0, len(recs))
		for _, v := range recs {
			out = append(out, map[string]any{"cnmtId": v.id, "cnmtType": v.typ, "recommendVersion": v.version})
		}
		log.Printf("[shop] pegasus recommend-versions %s -> %d record(s)", id, len(out))
		writeJSON(w, 200, map[string]any{"recommendVersions": out})
	default:
		// /v1/ratinglist: NIM fetches it into ns's rating list database; the body's
		// shape is not recovered, so it is refused rather than guessed.
		dump(r, body, "!! UNHANDLED pegasus")
		writeJSON(w, 404, map[string]any{"status": 404, "title": "not found"})
	}
}

// ---------- pushmo ----------

// pushmo serves NIM's download lists. <id> is not recovered either (a device id is the
// likeliest reading of the "%s"); the id is matched against device links, then account
// ids, then the requesting device, and the whole request is logged loudly.
func (s *server) pushmo(w http.ResponseWriter, r *http.Request, body []byte) {
	s.ensureShop()
	dump(r, body, "!! ESHOP pushmo (body shape unconfirmed; see eshop_nim.go)")
	p := r.URL.Path
	var kind, id string
	switch {
	case strings.HasPrefix(p, "/pushmo/d/") && strings.HasSuffix(p, ".json"):
		kind, id = "d", strings.TrimSuffix(strings.TrimPrefix(p, "/pushmo/d/"), ".json")
	case strings.HasPrefix(p, "/pushmo/p/") && strings.HasSuffix(p, ".json"):
		kind, id = "p", strings.TrimSuffix(strings.TrimPrefix(p, "/pushmo/p/"), ".json")
	default:
		writeJSON(w, 404, map[string]any{"status": 404, "title": "not found"})
		return
	}
	s.shop.reloadCatalog()
	var tasks []map[string]any
	if kind == "d" {
		tasks = s.pushmoTasks(s.pushmoAccounts(id, r))
	} else {
		tasks = s.pushmoTasks(s.penneShopAccounts(id))
	}
	log.Printf("[shop] pushmo %s/%s -> %d task(s)", kind, id, len(tasks))
	writeETagged(w, r, []byte(mustJSON(map[string]any{"tasks": tasks})))
}

// pushmoAccounts resolves a pushmo id to the accounts whose downloads it lists.
func (s *server) pushmoAccounts(id string, r *http.Request) []string {
	id = strings.ToLower(id)
	if a := s.shop.accountsForDevice(id); len(a) > 0 {
		return a
	}
	s.shop.mu.Lock()
	for _, l := range s.shop.Links.Links {
		if strings.ToLower(l.AccountID) == id {
			s.shop.mu.Unlock()
			return []string{l.AccountID}
		}
	}
	s.shop.mu.Unlock()
	return s.shop.accountsForDevice(s.deviceIdentity(r))
}

// pushmoTasks renders the open download tasks of the accounts in the download-task-list
// shape: one task per title, listing the application and, when published, its patch.
func (s *server) pushmoTasks(accounts []string) []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, account := range accounts {
		for _, task := range s.shop.tasksFor(account) {
			if task.State != "queued" && task.State != "downloading" && task.State != "paused" {
				continue
			}
			if seen[task.TitleID] {
				continue
			}
			t, _ := s.shop.meta(task.TitleID)
			if t == nil {
				continue
			}
			seen[task.TitleID] = true
			s.shop.mu.Lock()
			titles := []map[string]any{{"id": t.TitleID, "version": t.applicationVersion(), "type": "Application"}}
			if t.Patch != nil {
				titles = append(titles, map[string]any{"id": patchIDFor(t.TitleID), "version": t.Patch.Version, "type": "Patch"})
			}
			s.shop.mu.Unlock()
			out = append(out, map[string]any{
				"id": taskUUID(task.ID), "owner_application": t.TitleID, "titles": titles,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["id"].(string) < out[j]["id"].(string) })
	return out
}

// taskUUID formats a task's 32-hex id as the 36-character UUID the task list parser
// requires (ns rejects a task id of any other length).
func taskUUID(id string) string {
	if len(id) != 32 {
		sum := sha1.Sum([]byte(id))
		id = hex.EncodeToString(sum[:16])
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[:8], id[8:12], id[12:16], id[16:20], id[20:])
}

// ---------- veer ----------

// veer tells NIM which content vendor to use. OpenPak has one: the atum host this adapter
// serves, which NIM's vendor table names "akamai" on 22.x (/v5) and 23.x (/v6) alike.
// The TTL is a day; the policy does not change while the server runs.
func (s *server) veer(w http.ResponseWriter, r *http.Request, body []byte) {
	if r.URL.Path != "/v5/policy.json" && r.URL.Path != "/v6/policy.json" {
		dump(r, body, "!! UNHANDLED veer")
		writeJSON(w, 404, map[string]any{"status": 404, "title": "not found"})
		return
	}
	log.Printf("[shop] veer %s context=%q last_vendor=%q -> akamai (atum)", r.URL.Path, r.URL.Query().Get("context"), r.URL.Query().Get("last_vendor"))
	writeJSON(w, 200, map[string]any{
		"ttl":     86400,
		"weights": []any{map[string]any{"vendor": "akamai", "weight": 100}},
	})
}
