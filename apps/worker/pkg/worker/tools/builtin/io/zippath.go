package io

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
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

// zipLimits bound what one uploaded archive may expand to (zip bomb). An entry
// is read into memory whole and written to the uploads bucket, every file gets
// a document_files row, and the text of all text entries is kept in memory and
// then chunked and embedded. Each bound is above what one worker instance
// (512 MiB of memory, a 600-second request) can process, so only archives that
// would otherwise exhaust the instance, and with it every other job it runs,
// or run out the request are refused.
var zipLimits = struct {
	records    int    // central directory records (files and folders) to parse
	entries    int    // files in the archive
	entryBytes uint64 // uncompressed size of one file
	totalBytes uint64 // uncompressed size of all files
	textBytes  int    // text of the text files, kept in memory
}{
	records:    200_000,
	entries:    10_000,
	entryBytes: 128 << 20,
	totalBytes: 1 << 30,
	textBytes:  64 << 20,
}

// errZipTooLarge is returned for an archive beyond zipLimits.
var errZipTooLarge = errors.New("zip archive expands beyond what can be processed")

// zipDirectorySignature starts every central directory record.
var zipDirectorySignature = []byte("PK\x01\x02")

// checkZipRecords bounds the central directory before zip.NewReader parses
// it. NewReader keeps a File of a few hundred bytes for every directory
// record before checkZipLimits can look at them, so an archive packed with
// empty entries would take several times its own size in memory. Every
// record that NewReader parses starts with the directory signature, so the
// number of signatures in the archive bounds the records it would keep.
func checkZipRecords(archive []byte) error {
	if n := bytes.Count(archive, zipDirectorySignature); n > zipLimits.records {
		return fmt.Errorf("%w: more than %d directory records", errZipTooLarge, zipLimits.records)
	}
	return nil
}

// checkZipLimits checks the files of an archive, as its central directory
// declares them, against zipLimits before anything is extracted. archive/zip
// refuses to inflate a file past its declared size, so the declared sizes
// bound what extraction reads.
func checkZipLimits(files []*zip.File) error {
	count := 0
	var total uint64
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}
		count++
		if count > zipLimits.entries {
			return fmt.Errorf("%w: more than %d files", errZipTooLarge, zipLimits.entries)
		}
		if f.UncompressedSize64 > zipLimits.entryBytes {
			return fmt.Errorf("%w: a file expands to %d bytes (limit %d)", errZipTooLarge, f.UncompressedSize64, zipLimits.entryBytes)
		}
		total += f.UncompressedSize64
		if total > zipLimits.totalBytes {
			return fmt.Errorf("%w: the files expand to more than %d bytes", errZipTooLarge, zipLimits.totalBytes)
		}
	}
	return nil
}
