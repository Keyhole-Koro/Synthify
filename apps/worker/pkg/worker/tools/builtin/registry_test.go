package builtin

import (
	"errors"
	"reflect"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// factory builds a minimal core.Tool whose Name we can identify in
// assertions. The registry never inspects IOSchema or Run, only carries them,
// so leaving them zero is fine for these tests.
func factory(name string) Factory {
	return func(*base.Context) (core.Tool, error) {
		return core.Tool{Name: name}, nil
	}
}

func TestRegistry_BuildPreservesOrder(t *testing.T) {
	r := NewRegistry()
	// Register in one order...
	r.Register("c", factory("c"))
	r.Register("a", factory("a"))
	r.Register("b", factory("b"))

	// ...Build in a different, explicit order. The agent wiring is
	// order-sensitive, so Build must follow the name list, not registration
	// order or sort order (contract §7).
	got, err := r.Build(nil, []string{"b", "c", "a"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	want := []string{"b", "c", "a"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("pos %d = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestRegistry_UnknownToolIsError(t *testing.T) {
	r := NewRegistry()
	r.Register("known", factory("known"))
	if _, err := r.Build(nil, []string{"known", "missing"}); err == nil {
		t.Fatal("expected error for unknown tool, got nil")
	}
}

func TestRegistry_BuildPropagatesFactoryError(t *testing.T) {
	r := NewRegistry()
	r.Register("boom", func(*base.Context) (core.Tool, error) {
		return core.Tool{}, errors.New("construction failed")
	})
	if _, err := r.Build(nil, []string{"boom"}); err == nil {
		t.Fatal("expected factory error to propagate")
	}
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	r := NewRegistry()
	r.Register("dup", factory("dup"))
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	r.Register("dup", factory("dup"))
}

func TestRegistry_NilFactoryAndEmptyNamePanic(t *testing.T) {
	t.Run("nil factory", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic on nil factory")
			}
		}()
		NewRegistry().Register("x", nil)
	})
	t.Run("empty name", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic on empty name")
			}
		}()
		NewRegistry().Register("", factory("x"))
	})
}

// Registries are independent instances; deps captured by one must not leak to
// another (contract §3: non-global, per-orchestrator).
func TestRegistry_InstancesAreIndependent(t *testing.T) {
	r1 := NewRegistry()
	r1.Register("only_in_r1", factory("only_in_r1"))
	r2 := NewRegistry()
	// r2 must not see r1's registration: building it errors as unknown.
	if _, err := r2.Build(nil, []string{"only_in_r1"}); err == nil {
		t.Fatal("registry instances share state")
	}
}

func TestBuildReturnsProductionOrder(t *testing.T) {
	set, err := Build(&base.Context{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := make([]string, 0, len(set.Tools))
	for _, tool := range set.Tools {
		got = append(got, tool.Name)
	}
	want := []string{
		"semantic_chunking", "generate_knowledge_tree", "persist_knowledge_tree",
		"journal_add_task", "journal_update_task",
		"analyze_dependencies",
		"glossary_register", "glossary_lookup",
		"quality_critique", "semantic_search", "deduplicate_and_merge",
		"extract_table_data", "repair_encoding", "extract_text",
		"generate_brief", "generate_html_summary",
		"grep_search", "create_transform",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tool order = %#v, want %#v", got, want)
	}
	if len(set.Memories) != 3 {
		t.Fatalf("memories = %d, want 3", len(set.Memories))
	}
}
