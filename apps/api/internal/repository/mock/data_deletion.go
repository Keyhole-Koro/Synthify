package mock

import (
	"context"
	"sort"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

var _ repository.DocumentDeletionRepository = (*Store)(nil)

// GetDocumentDeletionState mirrors postgres.Store.GetDocumentDeletionState.
func (s *Store) GetDocumentDeletionState(ctx context.Context, documentID string) (*domain.DocumentDeletionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.documents[documentID]; !ok {
		return nil, domain.ErrNotFound
	}
	state := &domain.DocumentDeletionState{}
	var jobs []*domain.DocumentProcessingJob
	for _, job := range s.jobs {
		if job.DocumentID == documentID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].CreatedAt != jobs[j].CreatedAt {
			return jobs[i].CreatedAt < jobs[j].CreatedAt
		}
		return jobs[i].JobID < jobs[j].JobID
	})
	for _, job := range jobs {
		state.JobIDs = append(state.JobIDs, job.JobID)
	}
	if r, ok := s.reservations[documentID]; ok && r.Status == "reserved" && time.Now().Before(r.ExpiresAt) {
		state.UploadPending = true
	}
	return state, nil
}

// DeleteDocumentRows mirrors postgres.Store.DeleteDocumentRows, including the
// foreign-key cascades the database performs there. The mock keeps no job
// mutation log, so a node comes from a document only through its sources here.
func (s *Store) DeleteDocumentRows(ctx context.Context, documentID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, ok := s.documents[documentID]
	if !ok {
		return 0, domain.ErrNotFound
	}

	wsItems := s.items[doc.WorkspaceID]
	deleted := map[string]bool{}
	parentOf := map[string]string{}
	for id, item := range wsItems {
		parentOf[id] = item.ParentID
		cites, citesOther := false, false
		for _, source := range s.sources[id] {
			if source.DocumentID == documentID {
				cites = true
			} else {
				citesOther = true
			}
		}
		if cites && !citesOther {
			deleted[id] = true
		}
	}
	for id, parent := range domain.ReparentAroundDeleted(parentOf, deleted) {
		wsItems[id].ParentID = parent
	}
	for id := range deleted {
		delete(wsItems, id)
		delete(s.sources, id)
		delete(s.sourceRefs, id)
	}
	for itemID, sources := range s.sources {
		kept := make([]*domain.ItemSource, 0, len(sources))
		documents := map[string]bool{}
		for _, source := range sources {
			if source.DocumentID != documentID {
				kept = append(kept, source)
				documents[source.DocumentID] = true
			}
		}
		if len(kept) == len(sources) {
			continue
		}
		s.sources[itemID] = kept
		if item, ok := wsItems[itemID]; ok {
			item.CrossDocument = len(documents) > 1
		}
	}

	if r, ok := s.reservations[documentID]; ok && r.Status == "confirmed" {
		if account, ok := s.accounts[r.AccountID]; ok {
			account.StorageUsedBytes = max(account.StorageUsedBytes-r.ActualSize, 0)
		}
	}
	delete(s.reservations, documentID)
	delete(s.documents, documentID)
	delete(s.docFiles, documentID)
	delete(s.chunks, documentID)
	for jobID, job := range s.jobs {
		if job.DocumentID != documentID {
			continue
		}
		delete(s.jobs, jobID)
		delete(s.capabilities, jobID)
		delete(s.plans, jobID)
		delete(s.approvals, jobID)
		delete(s.checkpoints, jobID)
	}
	return len(deleted), nil
}
