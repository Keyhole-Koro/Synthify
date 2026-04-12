package mock

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/synthify/backend/internal/domain"
)

// Store はすべてのデータをメモリ上に保持するモックストア。
type Store struct {
	mu         sync.RWMutex
	workspaces map[string]*domain.Workspace
	members    map[string][]*domain.WorkspaceMember // workspace_id → members
	documents  map[string]*domain.Document          // document_id → document
	nodes      map[string][]*domain.Node            // document_id → nodes
	edges      map[string][]*domain.Edge            // document_id → edges
	views      map[string][]viewRecord              // "user_id:workspace_id" → view records
	aliases    map[string]string                    // alias node id -> canonical node id
}

type viewRecord struct {
	NodeID       string
	DocumentID   string
	LastViewedAt string
}

func NewStore() *Store {
	s := &Store{
		workspaces: make(map[string]*domain.Workspace),
		members:    make(map[string][]*domain.WorkspaceMember),
		documents:  make(map[string]*domain.Document),
		nodes:      make(map[string][]*domain.Node),
		edges:      make(map[string][]*domain.Edge),
		views:      make(map[string][]viewRecord),
		aliases:    make(map[string]string),
	}
	s.seed()
	return s
}

func newID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ─── Workspace ────────────────────────────────────────────────────────────────

func (s *Store) ListWorkspaces() []*domain.Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Workspace, 0, len(s.workspaces))
	for _, w := range s.workspaces {
		out = append(out, w)
	}
	return out
}

func (s *Store) GetWorkspace(id string) (*domain.Workspace, []*domain.WorkspaceMember, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.workspaces[id]
	if !ok {
		return nil, nil, false
	}
	return ws, s.members[id], true
}

func (s *Store) CreateWorkspace(name string) *domain.Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := &domain.Workspace{
		WorkspaceID:       newID("ws"),
		Name:              name,
		OwnerID:           "user_demo",
		Plan:              "free",
		StorageUsedBytes:  0,
		StorageQuotaBytes: 1 << 30, // 1GB
		MaxFileSizeBytes:  50 << 20,
		MaxUploadsPerDay:  10,
		CreatedAt:         now(),
	}
	s.workspaces[ws.WorkspaceID] = ws
	s.members[ws.WorkspaceID] = []*domain.WorkspaceMember{
		{UserID: "user_demo", Email: "demo@synthify.dev", Role: "owner", IsDev: true, InvitedAt: now()},
	}
	return ws
}

func (s *Store) InviteMember(wsID, email, role string, isDev bool) (*domain.WorkspaceMember, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.workspaces[wsID]; !ok {
		return nil, false
	}
	m := &domain.WorkspaceMember{
		UserID:    newID("user"),
		Email:     email,
		Role:      domain.WorkspaceRole(role),
		IsDev:     isDev,
		InvitedAt: now(),
		InvitedBy: "user_demo",
	}
	s.members[wsID] = append(s.members[wsID], m)
	return m, true
}

func (s *Store) UpdateMemberRole(wsID, userID, role string, isDev bool) (*domain.WorkspaceMember, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	members, ok := s.members[wsID]
	if !ok {
		return nil, false
	}
	for _, member := range members {
		if member.UserID == userID {
			member.Role = domain.WorkspaceRole(role)
			member.IsDev = isDev
			return member, true
		}
	}
	return nil, false
}

func (s *Store) RemoveMember(wsID, userID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	members, ok := s.members[wsID]
	if !ok {
		return false
	}
	for i, member := range members {
		if member.UserID == userID {
			s.members[wsID] = append(members[:i], members[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Store) TransferOwnership(wsID, newOwnerUserID string) (*domain.Workspace, []*domain.WorkspaceMember, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.workspaces[wsID]
	if !ok {
		return nil, nil, false
	}
	members, ok := s.members[wsID]
	if !ok {
		return nil, nil, false
	}
	found := false
	for _, member := range members {
		switch member.UserID {
		case newOwnerUserID:
			member.Role = domain.WorkspaceRoleOwner
			found = true
		case ws.OwnerID:
			member.Role = domain.WorkspaceRoleEditor
		}
	}
	if !found {
		return nil, nil, false
	}
	ws.OwnerID = newOwnerUserID
	return ws, members, true
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

func (s *Store) CreateDocument(wsID, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc := &domain.Document{
		DocumentID:  newID("doc"),
		WorkspaceID: wsID,
		UploadedBy:  "user_demo",
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		Status:      domain.DocumentLifecycleUploaded,
		CreatedAt:   now(),
		UpdatedAt:   now(),
	}
	s.documents[doc.DocumentID] = doc
	uploadURL := fmt.Sprintf("http://gcs:4443/synthify-uploads/%s/%s", wsID, doc.DocumentID)
	return doc, uploadURL
}

func (s *Store) GetUploadURL(wsID, filename, mimeType string, fileSize int64) (string, string) {
	token := newID("upload")
	uploadURL := fmt.Sprintf("http://gcs:4443/synthify-uploads/%s/%s/%s", wsID, token, filename)
	return uploadURL, token
}

func (s *Store) StartProcessing(docID string, forceReprocess bool, depth string) (*domain.Document, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, ok := s.documents[docID]
	if !ok {
		return nil, false
	}
	if depth == "" {
		depth = "full"
	}
	doc.Status = domain.DocumentLifecycleCompleted
	doc.ExtractionDepth = depth
	doc.CurrentStage = ""
	doc.UpdatedAt = now()

	// モック: 即時完了として sales ドキュメントのノード/エッジを複製して付与
	if _, exists := s.nodes[docID]; !exists {
		s.nodes[docID] = cloneSalesNodes(docID)
		s.edges[docID] = cloneSalesEdges(docID)
		doc.NodeCount = len(s.nodes[docID])
	}
	return doc, true
}

// ─── Graph ────────────────────────────────────────────────────────────────────

func (s *Store) GetGraph(docID string) ([]*domain.Node, []*domain.Edge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes, ok := s.nodes[docID]
	if !ok {
		return nil, nil, false
	}
	return nodes, s.edges[docID], true
}

func (s *Store) FindPaths(docID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes, ok := s.nodes[docID]
	if !ok {
		return nil, nil, nil, false
	}
	edges := s.edges[docID]
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
			path.Evidence.SourceDocumentIDs = []string{docID}
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

func (s *Store) CreateNode(docID, label, category, description, parentNodeID string, level int, createdBy string) *domain.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := &domain.Node{
		NodeID:      newID("nd"),
		DocumentID:  docID,
		Label:       label,
		Level:       level,
		Category:    category,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   now(),
	}
	s.nodes[docID] = append(s.nodes[docID], n)
	if parentNodeID != "" {
		e := &domain.Edge{
			EdgeID:       newID("ed"),
			DocumentID:   docID,
			SourceNodeID: parentNodeID,
			TargetNodeID: n.NodeID,
			EdgeType:     "hierarchical",
		}
		s.edges[docID] = append(s.edges[docID], e)
	}
	return n
}

func (s *Store) RecordView(userID, wsID, nodeID, docID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + ":" + wsID
	s.views[key] = append(s.views[key], viewRecord{
		NodeID:       nodeID,
		DocumentID:   docID,
		LastViewedAt: now(),
	})
}

func (s *Store) GetUserNodeActivity(wsID, userID, documentID string, limit int) domain.UserNodeActivity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}

	activity := domain.UserNodeActivity{
		UserID:      userID,
		DisplayName: userID,
	}
	for _, members := range s.members {
		for _, member := range members {
			if member.UserID == userID {
				activity.DisplayName = member.Email
			}
		}
	}

	viewMap := make(map[string]*domain.ViewedNodeEntry)
	for _, record := range s.views[userID+":"+wsID] {
		if documentID != "" && record.DocumentID != documentID {
			continue
		}
		entry := viewMap[record.NodeID]
		if entry == nil {
			entry = &domain.ViewedNodeEntry{
				NodeID:       record.NodeID,
				DocumentID:   record.DocumentID,
				Label:        s.lookupNodeLabel(record.NodeID),
				LastViewedAt: record.LastViewedAt,
			}
			viewMap[record.NodeID] = entry
		}
		entry.ViewCount++
		entry.LastViewedAt = record.LastViewedAt
	}
	for _, entry := range viewMap {
		activity.ViewedNodes = append(activity.ViewedNodes, *entry)
	}
	if len(activity.ViewedNodes) > limit {
		activity.ViewedNodes = activity.ViewedNodes[:limit]
	}

	for _, nodes := range s.nodes {
		for _, node := range nodes {
			if node.CreatedBy != userID {
				continue
			}
			if documentID != "" && node.DocumentID != documentID {
				continue
			}
			activity.CreatedNodes = append(activity.CreatedNodes, domain.CreatedNodeEntry{
				NodeID:     node.NodeID,
				DocumentID: node.DocumentID,
				Label:      node.Label,
				CreatedAt:  node.CreatedAt,
			})
		}
	}
	if len(activity.CreatedNodes) > limit {
		activity.CreatedNodes = activity.CreatedNodes[:limit]
	}
	return activity
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

func (s *Store) seed() {
}

func cloneSalesNodes(docID string) []*domain.Node {
	n := now()
	return []*domain.Node{
		{
			NodeID:      "nd_root",
			DocumentID:  docID,
			Label:       "販売戦略",
			Level:       0,
			Category:    "concept",
			Description: "当期における販売拡大の最上位方針",
			SummaryHTML: `<p>当期における販売拡大の最上位方針を定義する。</p>
<p>主要施策として <a data-paper-id="nd_tel">テレアポ施策</a> と <a data-paper-id="nd_sns">SNS施策</a> を採用し、短期と長期の両軸で成果を最大化する。</p>
<ul>
  <li><strong>短期目標</strong>: テレアポによる既存リードの掘り起こし</li>
  <li><strong>長期目標</strong>: SNS によるブランディングと新規顧客獲得</li>
</ul>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_tel",
			DocumentID:  docID,
			Label:       "テレアポ施策",
			Level:       1,
			Category:    "concept",
			Description: "月次100件を目標とした架電施策",
			SummaryHTML: `<p>月次100件を目標とした架電施策。<a data-paper-id="nd_cv">CV率 3.2%</a> を実績ベースラインとする。</p>
<p>効果を高めるために <a data-paper-id="nd_script">スクリプト改善</a> を並行実施する。</p>
<p>一方で <a data-paper-id="nd_counter">テレアポ不要論</a> という反論も存在する。</p>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_sns",
			DocumentID:  docID,
			Label:       "SNS施策",
			Level:       1,
			Category:    "concept",
			Description: "SNSを活用したブランド認知向上施策",
			SummaryHTML: `<p>SNS を活用したブランド認知向上施策。<a data-paper-id="nd_roi">ROI比較</a> に基づき優先度を決定している。</p>
<p><a data-paper-id="nd_evidence">A社事例</a> が有力な根拠となっている。</p>
<table>
  <thead><tr><th>プラットフォーム</th><th>月次予算</th></tr></thead>
  <tbody>
    <tr><td>LinkedIn</td><td>¥500,000</td></tr>
    <tr><td>X (Twitter)</td><td>¥300,000</td></tr>
  </tbody>
</table>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_cv",
			DocumentID:  docID,
			Label:       "CV率 3.2%",
			Level:       2,
			Category:    "entity",
			EntityType:  "metric",
			Description: "テレアポの成約率。前期比 +0.8pp の改善",
			SummaryHTML: `<p>テレアポの成約率を示す指標。前期比 +0.8pp の改善を達成した。</p>
<p>この数値は <a data-paper-id="nd_roi">ROI比較</a> の入力データとして使用されている。</p>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_script",
			DocumentID:  docID,
			Label:       "スクリプト改善",
			Level:       2,
			Category:    "concept",
			Description: "架電品質向上のためのトークスクリプト見直し施策",
			SummaryHTML: `<p>架電品質向上のためのトークスクリプト見直し施策。</p>
<ul>
  <li>導入部の簡略化（30秒以内）</li>
  <li>課題確認フェーズの追加</li>
  <li>クロージングの選択肢提示方式への変更</li>
</ul>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_roi",
			DocumentID:  docID,
			Label:       "ROI比較",
			Level:       2,
			Category:    "claim",
			Description: "テレアポとSNSの投資対効果比較。SNSのROIが2.3倍高い",
			SummaryHTML: `<p>テレアポと SNS の投資対効果を比較した結果、SNS の ROI が 2.3 倍高いことが示された。</p>
<table>
  <thead><tr><th>施策</th><th>ROI</th><th>CAC</th></tr></thead>
  <tbody>
    <tr><td>テレアポ</td><td>1.0x</td><td>¥120,000</td></tr>
    <tr><td>SNS</td><td>2.3x</td><td>¥52,000</td></tr>
  </tbody>
</table>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_counter",
			DocumentID:  docID,
			Label:       "テレアポ不要論",
			Level:       2,
			Category:    "counter",
			Description: "SNSのROIが高いことからテレアポへの投資削減を主張する反論",
			SummaryHTML: `<p>SNS の ROI が高いことから、<a data-paper-id="nd_tel">テレアポ施策</a> への投資を削減すべきという反論。</p>
<p>ただし関係構築面での強みは依然として存在するため、完全廃止は時期尚早との意見もある。</p>`,
			CreatedAt: n,
		},
		{
			NodeID:      "nd_evidence",
			DocumentID:  docID,
			Label:       "A社事例",
			Level:       2,
			Category:    "evidence",
			Description: "競合A社がSNSマーケティングを強化し、新規リード獲得180%を達成した事例",
			SummaryHTML: `<p>競合 A 社が SNS マーケティングを強化した結果、新規リード獲得数が前年比 180% に達した事例。</p>
<p>この事例は <a data-paper-id="nd_roi">ROI比較</a> の根拠として採用されている。</p>`,
			CreatedAt: n,
		},
	}
}

func cloneSalesEdges(docID string) []*domain.Edge {
	n := now()
	return []*domain.Edge{
		// hierarchical edges (ツリー構造)
		{EdgeID: "ed_01", DocumentID: docID, SourceNodeID: "nd_root", TargetNodeID: "nd_tel", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_02", DocumentID: docID, SourceNodeID: "nd_root", TargetNodeID: "nd_sns", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_03", DocumentID: docID, SourceNodeID: "nd_tel", TargetNodeID: "nd_cv", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_04", DocumentID: docID, SourceNodeID: "nd_tel", TargetNodeID: "nd_script", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_05", DocumentID: docID, SourceNodeID: "nd_tel", TargetNodeID: "nd_counter", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_06", DocumentID: docID, SourceNodeID: "nd_sns", TargetNodeID: "nd_roi", EdgeType: "hierarchical", CreatedAt: n},
		{EdgeID: "ed_07", DocumentID: docID, SourceNodeID: "nd_sns", TargetNodeID: "nd_evidence", EdgeType: "hierarchical", CreatedAt: n},
		// non-hierarchical edges (summary_html 内の data-paper-id リンクとして表現)
		{EdgeID: "ed_08", DocumentID: docID, SourceNodeID: "nd_cv", TargetNodeID: "nd_roi", EdgeType: "measured_by", Description: "CV率はROI比較の入力データ", CreatedAt: n},
		{EdgeID: "ed_09", DocumentID: docID, SourceNodeID: "nd_counter", TargetNodeID: "nd_tel", EdgeType: "contradicts", Description: "テレアポ不要論はテレアポ施策に反論する", CreatedAt: n},
		{EdgeID: "ed_10", DocumentID: docID, SourceNodeID: "nd_evidence", TargetNodeID: "nd_roi", EdgeType: "supports", Description: "A社事例はROI比較を支持する", CreatedAt: n},
	}
}

// Edge には CreatedAt が不要だが、将来的に使う可能性があるためフィールドを保持。
func init() {
	_ = now
}

func contains(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func (s *Store) workspaceExists(wsID string) bool {
	_, ok := s.workspaces[wsID]
	return ok
}

func (s *Store) nodeExists(nodeID string) bool {
	return s.lookupNodeLabel(nodeID) != ""
}

func (s *Store) lookupNodeLabel(nodeID string) string {
	for _, nodes := range s.nodes {
		for _, node := range nodes {
			if node.NodeID == nodeID {
				return node.Label
			}
		}
	}
	return ""
}
