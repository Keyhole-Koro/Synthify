package storage

import (
	"fmt"
	"strings"
)

// Object-name prefixes that hold a workspace's, a document's or a job's data in
// the uploads bucket. Deleting that data means deleting everything under them.
//
//	{ws}/{doc}/source/      uploaded original (API)
//	{ws}/{doc}/extracted/   worker's working copy / zip expansion
//	.cache/v1/{doc}/        worker's grep cache (excerpts of the document)
//	.checkpoints/{job}/     worker's stage checkpoints (stage outputs)
//
// The worker builds the last two in apps/worker/pkg/worker/storage (CachePath,
// CheckpointPath); its layout test keeps both sides in step. Every prefix ends
// in "/" so it never matches a sibling whose ID merely starts the same way.

const (
	cacheRoot      = ".cache"
	cacheVersion   = "v1"
	checkpointRoot = ".checkpoints"
)

// WorkspaceObjectPrefix returns "{ws}/": every document of the workspace.
func WorkspaceObjectPrefix(workspaceID string) string {
	return workspaceID + "/"
}

// DocumentObjectPrefix returns "{ws}/{doc}/": the document's original and its
// extracted files.
func DocumentObjectPrefix(workspaceID, documentID string) string {
	return workspaceID + "/" + DocumentPrefix(documentID)
}

// DocumentCachePrefix returns ".cache/v1/{doc}/": the worker's cache for one
// document.
func DocumentCachePrefix(documentID string) string {
	return cacheRoot + "/" + cacheVersion + "/" + documentID + "/"
}

// JobCheckpointPrefix returns ".checkpoints/{job}/": the worker's checkpoints
// for one processing job.
func JobCheckpointPrefix(jobID string) string {
	return checkpointRoot + "/" + jobID + "/"
}

// ValidateDeletionPrefix accepts only the four shapes above with non-empty IDs.
// Object deleters call it before listing anything, so a bug upstream (an empty
// ID, a shared directory) cannot turn into "delete every document".
func ValidateDeletionPrefix(prefix string) error {
	if !strings.HasSuffix(prefix, "/") {
		return fmt.Errorf("deletion prefix %q must end with /", prefix)
	}
	segments := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
	for _, segment := range segments {
		switch segment {
		case "", ".", "..":
			return fmt.Errorf("deletion prefix %q has an empty or relative segment", prefix)
		}
	}
	switch {
	case len(segments) == 1 && isOwnerID(segments[0]):
		return nil // {ws}/
	case len(segments) == 2 && isOwnerID(segments[0]) && isOwnerID(segments[1]):
		return nil // {ws}/{doc}/
	case len(segments) == 3 && segments[0] == cacheRoot && segments[1] == cacheVersion && isOwnerID(segments[2]):
		return nil // .cache/v1/{doc}/
	case len(segments) == 2 && segments[0] == checkpointRoot && isOwnerID(segments[1]):
		return nil // .checkpoints/{job}/
	default:
		return fmt.Errorf("deletion prefix %q is not one workspace's, document's or job's data", prefix)
	}
}

// isOwnerID reports whether a path segment can be a workspace, document or job
// ID. Real IDs are ULIDs; anything starting with "." is a shared directory.
func isOwnerID(segment string) bool {
	return segment != "" && !strings.HasPrefix(segment, ".")
}
