package jobstatus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator"
)

// ReaderAccess keeps, for each workspace, the users who may read its job
// status documents (workspaces/{workspaceId}/jobs/{jobId}).
//
// The web app reads those documents straight from Firestore. Who may access a
// workspace is decided in Postgres (the users of the workspace's account and
// its workspace_members), which the security rules cannot see. The API
// therefore mirrors that access as workspaces/{workspaceId}/readers/{userId}
// documents, and firestore.rules lets a signed-in user read a workspace's job
// status documents only while such a document exists for them. Clients can
// neither read nor write the reader documents.
type ReaderAccess interface {
	// Grant lets userID read the job status documents of each workspace.
	Grant(ctx context.Context, userID string, workspaceIDs ...string) error
	// Revoke takes that away from userID for one workspace.
	Revoke(ctx context.Context, userID, workspaceID string) error
	// RevokeAll takes it away from every user of the workspace.
	RevokeAll(ctx context.Context, workspaceID string) error
}

const readersCollection = "readers"

// NewReaderAccess returns the ReaderAccess for the Firestore of projectID. Like
// NewNotifier, it returns one that does nothing when projectID is empty or the
// client cannot be created. Doing nothing can only take access away: without
// reader documents the rules let nobody read.
func NewReaderAccess(ctx context.Context, projectID string, logger *slog.Logger) ReaderAccess {
	if projectID == "" {
		return noopReaderAccess{}
	}
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	app, err := firebase.NewApp(initCtx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		logger.Error("jobstatus.readers_firebase_init_failed", "error", err.Error(), "project_id", projectID)
		return noopReaderAccess{}
	}
	client, err := app.Firestore(initCtx)
	if err != nil {
		logger.Error("jobstatus.readers_firestore_init_failed", "error", err.Error(), "project_id", projectID)
		return noopReaderAccess{}
	}
	return NewFirestoreReaderAccess(client)
}

// NewFirestoreReaderAccess returns the ReaderAccess that writes the reader
// documents with client.
func NewFirestoreReaderAccess(client *firestore.Client) ReaderAccess {
	return &firestoreReaderAccess{client: client}
}

type firestoreReaderAccess struct {
	client *firestore.Client
}

func (a *firestoreReaderAccess) readers(workspaceID string) *firestore.CollectionRef {
	return a.client.Collection("workspaces").Doc(workspaceID).Collection(readersCollection)
}

func (a *firestoreReaderAccess) Grant(ctx context.Context, userID string, workspaceIDs ...string) error {
	if err := checkDocID("user id", userID); err != nil {
		return err
	}
	for _, wsID := range workspaceIDs {
		if err := checkDocID("workspace id", wsID); err != nil {
			return err
		}
	}
	if len(workspaceIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	bw := a.client.BulkWriter(ctx)
	jobs := make([]*firestore.BulkWriterJob, 0, len(workspaceIDs))
	var errs []error
	seen := make(map[string]bool, len(workspaceIDs))
	for _, wsID := range workspaceIDs {
		// BulkWriter refuses a second write to the same document.
		if seen[wsID] {
			continue
		}
		seen[wsID] = true
		job, err := bw.Set(a.readers(wsID).Doc(userID), map[string]any{
			"userId":      userID,
			"workspaceId": wsID,
			"grantedAt":   now,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("grant %s on %s: %w", userID, wsID, err))
			continue
		}
		jobs = append(jobs, job)
	}
	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a *firestoreReaderAccess) Revoke(ctx context.Context, userID, workspaceID string) error {
	if err := checkDocID("user id", userID); err != nil {
		return err
	}
	if err := checkDocID("workspace id", workspaceID); err != nil {
		return err
	}
	// Deleting a document that does not exist succeeds.
	_, err := a.readers(workspaceID).Doc(userID).Delete(ctx)
	return err
}

func (a *firestoreReaderAccess) RevokeAll(ctx context.Context, workspaceID string) error {
	if err := checkDocID("workspace id", workspaceID); err != nil {
		return err
	}
	refs := a.readers(workspaceID).DocumentRefs(ctx)
	bw := a.client.BulkWriter(ctx)
	var jobs []*firestore.BulkWriterJob
	var errs []error
	for {
		ref, err := refs.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("list readers of %s: %w", workspaceID, err))
			break
		}
		job, err := bw.Delete(ref)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		jobs = append(jobs, job)
	}
	bw.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// checkDocID rejects IDs that cannot name a single Firestore document.
func checkDocID(field, id string) error {
	if id == "" || id == "." || id == ".." || strings.Contains(id, "/") {
		return fmt.Errorf("jobstatus: invalid %s %q", field, id)
	}
	return nil
}

type noopReaderAccess struct{}

func (noopReaderAccess) Grant(context.Context, string, ...string) error { return nil }
func (noopReaderAccess) Revoke(context.Context, string, string) error   { return nil }
func (noopReaderAccess) RevokeAll(context.Context, string) error        { return nil }
