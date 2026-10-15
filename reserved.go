package main

// Reserved titles: the names a friend's presence can carry that are not games.
//
// A friend who is live on Pier, or Harbor, or a PS3, is listed on a Switch by the id of a title,
// and a console that does not have that title asks atum for its control NCA (name and icon) and
// draws what it gets. These ids -- 01fe00f<generation><index>000, built by hbstore-tools/reserved.py -- are
// not in the catalog: a file $NX_DATA/shop/reserved/<id>.nca is all there is, so they show in no
// storefront, update list or library, and need no ticket, version or content record.

import (
	"os"
	"path/filepath"
	"strings"
)

// reservedPrefix is the part of the id every reserved title shares: 01fe00f and a generation digit
// (01fe00ff was the first, 01fe00fe the second). A console never asks again for an id it has, so a
// new name or picture is a new generation; the old files stay and keep answering.
const reservedPrefix = "01fe00f"

// reservedControl is the control NCA of a reserved title, or "" when the id is not one or has no
// file. Only the control route uses it: there is nothing to install.
func (s *shopState) reservedControl(titleID string) string {
	id := strings.ToLower(titleID)
	if s.dir == "" || !strings.HasPrefix(id, reservedPrefix) || !validTitleID(id) {
		return ""
	}
	p := filepath.Join(s.dir, "reserved", id+".nca")
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return ""
	}
	return p
}
