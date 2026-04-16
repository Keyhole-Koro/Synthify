package mock

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Keyhole-Koro/SynthifyShared/domain"
	"github.com/Keyhole-Koro/SynthifyShared/repository"
	"github.com/oklog/ulid/v2"
)

// Store is an in-memory mock store that holds all data.
type Store struct {
	mu                 sync.RWMutex
	accounts           map[string]*domain.Account               // account_id → account
	accountUsers       map[string][]string                      // account_id → []user_id
	userAccount        map[string]string                        // user_id → account_id
	workspaces         map[string]*domain.Workspace             // workspace_id → workspace
	documents          map[string]*domain.Document              // document_id → document
	documentChunks     map[string][]*domain.DocumentChunk       // document_id → chunks
	jobs               map[string]*domain.DocumentProcessingJob // job_id → job
	graphs             map[string]*domain.Graph                 // workspace_id → graph
	nodes              map[string][]*domain.Node                // graph_id → nodes
	edges              map[string][]*domain.Edge                // graph_id → edges
	aliases            map[string]string                        // alias node id -> canonical node id
	uploadURLGenerator repository.UploadURLGenerator
}

func NewStore(uploadURLGenerator ...repository.UploadURLGenerator) *Store {
	var generator repository.UploadURLGenerator
	if len(uploadURLGenerator) > 0 && uploadURLGenerator[0] != nil {
		generator = uploadURLGenerator[0]
	} else {
		generator = func(workspaceID, documentID string) string {
			return fmt.Sprintf("http://example.local/%s/%s", workspaceID, documentID)
		}
	}
	s := &Store{
		accounts:           make(map[string]*domain.Account),
		accountUsers:       make(map[string][]string),
		userAccount:        make(map[string]string),
		workspaces:         make(map[string]*domain.Workspace),
		documents:          make(map[string]*domain.Document),
		documentChunks:     make(map[string][]*domain.DocumentChunk),
		jobs:               make(map[string]*domain.DocumentProcessingJob),
		graphs:             make(map[string]*domain.Graph),
		nodes:              make(map[string][]*domain.Node),
		edges:              make(map[string][]*domain.Edge),
		aliases:            make(map[string]string),
		uploadURLGenerator: generator,
	}
	return s
}

func newID() string {
	return ulid.Make().String()
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ─── Account ──────────────────────────────────────────────────────────────────

func (s *Store) GetOrCreateAccount(userID string) (*domain.Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if accountID, ok := s.userAccount[userID]; ok {
		if acct, ok := s.accounts[accountID]; ok {
			return acct, nil
		}
	}
	acct := &domain.Account{
		AccountID:          newID(),
		Name:               fmt.Sprintf("account-%s", userID),
		Plan:               "registered",
		StorageQuotaBytes:  5 * 1 << 30,
		StorageUsedBytes:   0,
		MaxFileSizeBytes:   100 << 20,
		MaxUploadsPerFiveH: 20,
		MaxUploadsPerWeek:  100,
		CreatedAt:          now(),
	}
	s.accounts[acct.AccountID] = acct
	s.accountUsers[acct.AccountID] = []string{userID}
	s.userAccount[userID] = acct.AccountID
	return acct, nil
}

func (s *Store) GetAccount(accountID string) (*domain.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	acct, ok := s.accounts[accountID]
	if !ok {
		return nil, errors.New("account not found")
	}
	return acct, nil
}

// ─── Workspace ────────────────────────────────────────────────────────────────

func (s *Store) ListWorkspacesByUser(userID string) []*domain.Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	accountID, ok := s.userAccount[userID]
	if !ok {
		return nil
	}
	var out []*domain.Workspace
	for _, ws := range s.workspaces {
		if ws.AccountID == accountID {
			out = append(out, ws)
		}
	}
	return out
}

func (s *Store) GetWorkspace(id string) (*domain.Workspace, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.workspaces[id]
	return ws, ok
}

func (s *Store) IsWorkspaceAccessible(wsID, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.workspaces[wsID]
	if !ok {
		return false
	}
	accountID, ok := s.userAccount[userID]
	if !ok {
		return false
	}
	return ws.AccountID == accountID
}

func (s *Store) CreateWorkspace(accountID, name string) *domain.Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := &domain.Workspace{
		WorkspaceID: newID(),
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   now(),
	}
	s.workspaces[ws.WorkspaceID] = ws
	return ws
}

// ─── Document ─────────────────────────────────────────────────────────────────

func (s *Store) ListDocuments(wsID string) []*domain.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*domain.Document
	for _, d := range s.documents {
		if d.WorkspaceID == wsID {
			out = append(out, d)
		}
	}
	return out
}

func (s *Store) GetDocument(id string) (*domain.Document, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.documents[id]
	return d, ok
}

func (s *Store) CreateDocument(wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := &domain.Document{
		DocumentID:  newID(),
		WorkspaceID: wsID,
		UploadedBy:  uploadedBy,
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		CreatedAt:   now(),
	}
	s.documents[doc.DocumentID] = doc
	return doc, s.uploadURLGenerator(wsID, doc.DocumentID)
}

func (s *Store) GetLatestProcessingJob(docID string) (*domain.DocumentProcessingJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *domain.DocumentProcessingJob
	for _, job := range s.jobs {
		if job.DocumentID == docID {
			if latest == nil || job.CreatedAt > latest.CreatedAt {
				latest = job
			}
		}
	}
	if latest == nil {
		return nil, false
	}
	return latest, true
}

func (s *Store) CreateProcessingJob(docID, graphID, jobType string) *domain.DocumentProcessingJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	job := &domain.DocumentProcessingJob{
		JobID:      newID(),
		DocumentID: docID,
		GraphID:    graphID,
		JobType:    jobType,
		Status:     "queued",
		CreatedAt:  now(),
		UpdatedAt:  now(),
	}
	s.jobs[job.JobID] = job

	// Mock behavior: immediately attach sample nodes and edges to the graph.
	if graphID != "" {
		if _, exists := s.nodes[graphID]; !exists {
			s.nodes[graphID] = cloneSalesNodes(graphID)
			s.edges[graphID] = cloneSalesEdges(graphID)
		}
	}
	return job
}

func (s *Store) MarkProcessingJobRunning(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return false
	}
	job.Status = "running"
	job.ErrorMessage = ""
	job.UpdatedAt = now()
	return true
}

func (s *Store) UpdateProcessingJobStage(jobID, stage string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return false
	}
	job.CurrentStage = stage
	job.UpdatedAt = now()
	return true
}

func (s *Store) FailProcessingJob(jobID, errorMessage string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return false
	}
	job.Status = "failed"
	job.ErrorMessage = errorMessage
	job.UpdatedAt = now()
	return true
}

func (s *Store) CompleteProcessingJob(jobID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return false
	}
	job.Status = "completed"
	job.UpdatedAt = now()
	return true
}

func (s *Store) SaveDocumentChunks(documentID string, chunks []*domain.DocumentChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documentChunks[documentID] = chunks
	return nil
}

// ─── Graph ────────────────────────────────────────────────────────────────────

func (s *Store) GetOrCreateGraph(wsID string) (*domain.Graph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.graphs {
		if g.WorkspaceID == wsID {
			return g, nil
		}
	}
	g := &domain.Graph{
		GraphID:     newID(),
		WorkspaceID: wsID,
		Name:        "default",
		CreatedAt:   now(),
		UpdatedAt:   now(),
	}
	s.graphs[g.GraphID] = g
	return g, nil
}

func (s *Store) GetGraphByWorkspace(wsID string) ([]*domain.Node, []*domain.Edge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var graphID string
	for _, g := range s.graphs {
		if g.WorkspaceID == wsID {
			graphID = g.GraphID
			break
		}
	}
	if graphID == "" {
		return nil, nil, false
	}
	return s.nodes[graphID], s.edges[graphID], true
}

func (s *Store) FindPaths(graphID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool) {
	s.mu.RLock()
	nodes := s.nodes[graphID]
	edges := s.edges[graphID]
	s.mu.RUnlock()

	if len(nodes) == 0 {
		return nil, nil, nil, false
	}
	if maxDepth <= 0 {
		maxDepth = 4
	}
	if limit <= 0 {
		limit = 3
	}

	nodeByID := make(map[string]*domain.Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.NodeID] = node
	}
	if nodeByID[sourceNodeID] == nil || nodeByID[targetNodeID] == nil {
		return nil, nil, nil, false
	}

	adj := make(map[string][]string)
	for _, edge := range edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		adj[edge.TargetNodeID] = append(adj[edge.TargetNodeID], edge.SourceNodeID)
	}

	type item struct {
		nodeID string
		path   []string
	}
	queue := []item{{nodeID: sourceNodeID, path: []string{sourceNodeID}}}
	var paths []domain.GraphPath
	seenPaths := make(map[string]bool)

	for len(queue) > 0 && len(paths) < limit {
		cur := queue[0]
		queue = queue[1:]
		if len(cur.path)-1 > maxDepth {
			continue
		}
		if cur.nodeID == targetNodeID {
			key := fmt.Sprint(cur.path)
			if seenPaths[key] {
				continue
			}
			seenPaths[key] = true
			path := domain.GraphPath{
				NodeIDs:  append([]string(nil), cur.path...),
				HopCount: len(cur.path) - 1,
			}
			paths = append(paths, path)
			continue
		}
		for _, next := range adj[cur.nodeID] {
			if contains(cur.path, next) {
				continue
			}
			nextPath := append(append([]string(nil), cur.path...), next)
			queue = append(queue, item{nodeID: next, path: nextPath})
		}
	}

	return nodes, edges, paths, true
}

// ─── Node ─────────────────────────────────────────────────────────────────────

func (s *Store) GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, nodes := range s.nodes {
		for _, n := range nodes {
			if n.NodeID == nodeID {
				var related []*domain.Edge
				for _, edges := range s.edges {
					for _, e := range edges {
						if e.SourceNodeID == nodeID || e.TargetNodeID == nodeID {
							related = append(related, e)
						}
					}
				}
				return n, related, true
			}
		}
	}
	return nil, nil, false
}

func (s *Store) CreateNode(graphID, label, description, parentNodeID, createdBy string) *domain.Node {
	node := s.CreateStructuredNode(graphID, label, "", 0, "", description, "", createdBy)
	if node == nil || parentNodeID == "" {
		return node
	}
	if s.CreateEdge(graphID, parentNodeID, node.NodeID, "hierarchical", "") == nil {
		return nil
	}
	return node
}

func (s *Store) CreateStructuredNode(graphID, label, category string, level int, entityType, description, summaryHTML, createdBy string) *domain.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := &domain.Node{
		NodeID:      newID(),
		GraphID:     graphID,
		Label:       label,
		Category:    category,
		Level:       level,
		EntityType:  entityType,
		Description: description,
		SummaryHTML: summaryHTML,
		CreatedBy:   createdBy,
		CreatedAt:   now(),
	}
	s.nodes[graphID] = append(s.nodes[graphID], n)
	return n
}

func (s *Store) UpsertNodeSource(_, _, _, _ string, _ float64) error {
	// No-op in the mock store.
	return nil
}

func (s *Store) CreateEdge(graphID, sourceNodeID, targetNodeID, edgeType, description string) *domain.Edge {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &domain.Edge{
		EdgeID:       newID(),
		GraphID:      graphID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		EdgeType:     edgeType,
		Description:  description,
		CreatedAt:    now(),
	}
	s.edges[graphID] = append(s.edges[graphID], e)
	return e
}

func (s *Store) UpsertEdgeSource(_, _, _, _ string, _ float64) error {
	return nil
}

func (s *Store) UpdateNodeSummaryHTML(nodeID, summaryHTML string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, nodes := range s.nodes {
		for _, node := range nodes {
			if node.NodeID == nodeID {
				node.SummaryHTML = summaryHTML
				return true
			}
		}
	}
	return false
}

func (s *Store) ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.workspaceExists(wsID) || !s.nodeExists(canonicalNodeID) || !s.nodeExists(aliasNodeID) {
		return false
	}
	s.aliases[aliasNodeID] = canonicalNodeID
	return true
}

func (s *Store) RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.workspaceExists(wsID) || !s.nodeExists(canonicalNodeID) || !s.nodeExists(aliasNodeID) {
		return false
	}
	delete(s.aliases, aliasNodeID)
	return true
}

// ─── Seed data ────────────────────────────────────────────────────────────────

func cloneSalesNodes(graphID string) []*domain.Node {
	n := now()
	return []*domain.Node{
		{NodeID: "nd_root", GraphID: graphID, Label: "販売戦略", Description: "当期における販売拡大の最上位方針", CreatedAt: n},
		{NodeID: "nd_tel", GraphID: graphID, Label: "テレアポ施策", Description: "月次100件を目標とした架電施策", CreatedAt: n},
		{NodeID: "nd_sns", GraphID: graphID, Label: "SNS施策", Description: "SNSを活用したブランド認知向上施策", CreatedAt: n},
		{NodeID: "nd_cv", GraphID: graphID, Label: "CV率 3.2%", EntityType: "metric", Description: "テレアポの成約率", CreatedAt: n},
		{NodeID: "nd_script", GraphID: graphID, Label: "スクリプト改善", Description: "架電品質向上のためのトークスクリプト見直し施策", CreatedAt: n},
		{NodeID: "nd_roi", GraphID: graphID, Label: "ROI比較", Description: "テレアポとSNSの投資対効果比較。SNSのROIが2.3倍高い", CreatedAt: n},
		{NodeID: "nd_counter", GraphID: graphID, Label: "テレアポ不要論", Description: "SNSのROIが高いことからテレアポへの投資削減を主張する反論", CreatedAt: n},
		{NodeID: "nd_evidence", GraphID: graphID, Label: "A社事例", Description: "競合A社がSNSマーケティングを強化し、新規リード獲得180%を達成した事例", CreatedAt: n},
	}
}

func cloneSalesEdges(graphID string) []*domain.Edge {
	n := now()
	return []*domain.Edge{
		{EdgeID: "ed_01", GraphID: graphID, SourceNodeID: "nd_root", TargetNodeID: "nd_tel", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_02", GraphID: graphID, SourceNodeID: "nd_root", TargetNodeID: "nd_sns", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_03", GraphID: graphID, SourceNodeID: "nd_tel", TargetNodeID: "nd_cv", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_04", GraphID: graphID, SourceNodeID: "nd_tel", TargetNodeID: "nd_script", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_05", GraphID: graphID, SourceNodeID: "nd_tel", TargetNodeID: "nd_counter", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_06", GraphID: graphID, SourceNodeID: "nd_sns", TargetNodeID: "nd_roi", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_07", GraphID: graphID, SourceNodeID: "nd_sns", TargetNodeID: "nd_evidence", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_08", GraphID: graphID, SourceNodeID: "nd_cv", TargetNodeID: "nd_roi", EdgeType: "measured_by", CreatedAt: n},
		{EdgeID: "ed_09", GraphID: graphID, SourceNodeID: "nd_counter", TargetNodeID: "nd_tel", EdgeType: "contradicts", CreatedAt: n},
		{EdgeID: "ed_10", GraphID: graphID, SourceNodeID: "nd_evidence", TargetNodeID: "nd_roi", EdgeType: "supports", CreatedAt: n},
	}
}

func (s *Store) workspaceExists(wsID string) bool {
	_, ok := s.workspaces[wsID]
	return ok
}

func (s *Store) nodeExists(nodeID string) bool {
	for _, nodes := range s.nodes {
		for _, node := range nodes {
			if node.NodeID == nodeID {
				return true
			}
		}
	}
	return false
}

func contains(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
