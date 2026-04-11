package postgres

import (
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFindPaths_FindsPathBetweenNodes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}
	createdAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)

	nodeRows := sqlmock.NewRows([]string{
		"node_id", "document_id", "label", "level", "category", "entity_type", "description", "summary_html", "created_by", "created_at",
	}).
		AddRow("nd_root", "doc_1", "Root", 0, "section", "", "root", "<p>root</p>", "user_demo", createdAt).
		AddRow("nd_child", "doc_1", "Child", 1, "concept", "", "child", "<p>child</p>", "user_demo", createdAt).
		AddRow("nd_target", "doc_1", "Target", 2, "concept", "", "target", "<p>target</p>", "user_demo", createdAt)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at
		FROM nodes
		WHERE document_id = $1
		ORDER BY level ASC, created_at ASC
	`)).
		WithArgs("doc_1").
		WillReturnRows(nodeRows)

	edgeRows := sqlmock.NewRows([]string{
		"edge_id", "document_id", "source_node_id", "target_node_id", "edge_type", "description", "created_at",
	}).
		AddRow("ed_1", "doc_1", "nd_root", "nd_child", "hierarchical", "", createdAt).
		AddRow("ed_2", "doc_1", "nd_child", "nd_target", "reference", "", createdAt)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at
		FROM edges
		WHERE document_id = $1
		ORDER BY created_at ASC
	`)).
		WithArgs("doc_1").
		WillReturnRows(edgeRows)

	nodes, edges, paths, ok := store.FindPaths("doc_1", "nd_root", "nd_target", 3, 2)
	if !ok {
		t.Fatal("FindPaths returned ok=false")
	}
	if len(nodes) != 3 {
		t.Fatalf("len(nodes) = %d, want 3", len(nodes))
	}
	if len(edges) != 2 {
		t.Fatalf("len(edges) = %d, want 2", len(edges))
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
	if got := paths[0].NodeIDs; len(got) != 3 || got[0] != "nd_root" || got[1] != "nd_child" || got[2] != "nd_target" {
		t.Fatalf("unexpected path node IDs: %#v", got)
	}
	if paths[0].HopCount != 2 {
		t.Fatalf("paths[0].HopCount = %d, want 2", paths[0].HopCount)
	}
	if got := paths[0].Evidence.SourceDocumentIDs; len(got) != 1 || got[0] != "doc_1" {
		t.Fatalf("unexpected path evidence: %#v", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
