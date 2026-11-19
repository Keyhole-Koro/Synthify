package io

import (
	"path/filepath"
	"strings"
)

// zipEntryPath maps a zip entry name to the file it is extracted to inside
// extractDir. Entry names come from the uploaded archive, so they are not
// trusted: a name like "../../../ws_other/doc/source/original.txt" would climb
// out of this document's working tree and overwrite files elsewhere on the
// shared mount (Zip Slip). ok is false for any name that is not a local path
// (absolute, empty, or escaping with "..") and for a name that resolves to
// extractDir itself; the caller skips such entries.
func zipEntryPath(extractDir, name string) (dest string, ok bool) {
	if !filepath.IsLocal(name) {
		return "", false
	}
	dest = filepath.Join(extractDir, name)
	rel, err := filepath.Rel(extractDir, dest)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return dest, true
}
