package main

import (
	"context"
	"encoding/json"
	"testing"
)

// Both firmware shapes go out, each to its own receiver, and each body is valid JSON of the
// VersionList type with an empty tag (always refreshes).
func TestPushVersionListSendsBothFirmwareShapes(t *testing.T) {
	p := newFakePenne()
	s := &server{penne: p}
	if n := s.pushVersionList(context.Background()); n != 2 {
		t.Fatalf("connections = %d, want 2", n)
	}
	want := map[string]string{"versionlist": "e_tag", "versionlist2": "eTag"}
	for _, m := range p.sent {
		var b map[string]any
		if err := json.Unmarshal([]byte(m.Body), &b); err != nil || b["type"] != "VersionList" || b[want[m.Receivers]] != "" {
			t.Fatalf("%s body %s: %v", m.Receivers, m.Body, err)
		}
		delete(want, m.Receivers)
	}
	if len(want) != 0 {
		t.Fatalf("receivers not sent: %v", want)
	}
}
