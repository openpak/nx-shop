package main

// version_push.go: tell every connected console the version list changed, so a new
// version shows up now instead of at NIM's next poll. NIM can't listen on NPNS itself;
// ns can: it takes a VersionList push, and NIM then re-fetches the list. The receiver and
// the body fields changed between firmwares, so both are sent and a console ignores the
// one it doesn't listen on:
//   - 22.x: receiver "versionlist", snake_case body. NOT verified on hardware.
//   - 23.x: receiver "versionlist2", camelCase body. Verified on 23.0.1 (2026-10-15).
// Either needs ns.notification!enable_version_list=1 and debug_version_list_waiting_limit=0
// in system_settings.ini (openpak-nro writes both); an empty tag always refreshes.

import (
	"context"
	"crypto/sha256"
	"log"
	"time"

	"github.com/openpak/nx-common/nxapi"
)

const (
	versionPushBody22 = `{"type":"VersionList","e_tag":"","wait_policy":{"type":"random","limit":0,"download_list":{"default":{"type":"random","minimum":0,"maximum":0}},"download_content":{"default":{"type":"random","minimum":0,"maximum":0}}}}`
	versionPushBody23 = `{"type":"VersionList","eTag":"","waitPolicy":{"type":"random","limit":0,"downloadList":{"default":{"type":"random","minimum":0,"maximum":0}},"downloadContent":{"default":{"type":"random","minimum":0,"maximum":0}}}}`
)

// pushVersionList broadcasts both message shapes; it returns the connections that took them.
func (s *server) pushVersionList(ctx context.Context) int {
	n := 0
	for _, m := range []nxapi.DeviceMessage{
		{Name: "versionlist", Receivers: "versionlist", Body: versionPushBody22},
		{Name: "versionlist2", Receivers: "versionlist2", Body: versionPushBody23},
	} {
		r, err := s.penne.Broadcast(ctx, m)
		if err != nil {
			log.Printf("[shop] version list push %s: %v", m.Name, err)
			continue
		}
		n += r.Connections
	}
	return n
}

// watchVersionList pushes whenever the served list changes. It polls: the catalog is a
// file the admin API or an operator edits, with no single write path to hook.
// ponytail: 30 s polling, a push per change; hook the writers if the delay ever matters.
func (s *server) watchVersionList(every time.Duration) {
	var last [32]byte
	for first := true; ; first = false {
		doc, _ := s.versionList(true)
		sum := sha256.Sum256([]byte(mustJSON(doc)))
		if !first && sum != last {
			n := s.pushVersionList(context.Background())
			log.Printf("[shop] version list changed: pushed to %d connection(s)", n)
		}
		last = sum
		time.Sleep(every)
	}
}
