package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core"
)

// JournalStatus indicates the current progress status of a task checklist item.
type JournalStatus string

const (
	// JournalStatusPending indicates a task that has not yet been started.
	JournalStatusPending JournalStatus = "pending"
	// JournalStatusInProgress indicates a task currently being worked on.
	JournalStatusInProgress JournalStatus = "in_progress"
	// JournalStatusCompleted indicates a task that has finished execution.
	JournalStatusCompleted JournalStatus = "completed"
)

// Task represents a single unit of work in the agent's execution checklist.
type Task struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Status      JournalStatus `json:"status"` // "pending", "in_progress", "completed"
	DependsOn   []string      `json:"depends_on,omitempty"`
}

// Journal manages an in-memory checklist of tasks for tracking progress through synthesis steps.
type Journal struct {
	mu    sync.RWMutex
	tasks []Task
}

// NewJournal initializes an empty Journal.
func NewJournal() *Journal {
	return &Journal{}
}

// RenderForPrompt formats the checklist of tasks as Markdown with checkboxes for prompt injection.
func (j *Journal) RenderForPrompt() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if len(j.tasks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("### Tasks\n")
	for _, t := range j.tasks {
		switch t.Status {
		case JournalStatusCompleted:
			sb.WriteString("- [x] ")
		case JournalStatusInProgress:
			sb.WriteString("- [~] ")
		default:
			sb.WriteString("- [ ] ")
		}
		sb.WriteString(t.ID)
		sb.WriteString(": ")
		sb.WriteString(t.Description)
		sb.WriteString("\n")
	}
	return sb.String()
}

// addArgs specifies parameters for adding a new task to the journal.
type addArgs struct {
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

// addResult contains the newly allocated task ID and confirmation message.
type addResult struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// NewAddTaskTool creates a core.Tool enabling an agent to record planning tasks.
func NewAddTaskTool(j *Journal) (core.Tool, error) {
	schema, err := core.IOSchemaFor[addArgs, addResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args addArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		id := fmt.Sprintf("task_%d", len(j.tasks)+1)
		j.tasks = append(j.tasks, Task{
			ID:          id,
			Description: args.Description,
			Status:      JournalStatusPending,
			DependsOn:   args.DependsOn,
		})
		out, mErr := json.Marshal(addResult{TaskID: id, Message: "Task added: " + id})
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "journal_add_task",
		Description: "Adds a new task to the processing checklist. Use this during planning to map out what needs to be done.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}

// updateArgs specifies the task to update and its new status.
type updateArgs struct {
	TaskID string        `json:"task_id"`
	Status JournalStatus `json:"status"`
}

// updateResult reports the outcome of the status update operation.
type updateResult struct {
	Message string `json:"message"`
}

// NewUpdateTaskTool creates a core.Tool enabling an agent to change task status in the journal.
func NewUpdateTaskTool(j *Journal) (core.Tool, error) {
	schema, err := core.IOSchemaFor[updateArgs, updateResult]()
	if err != nil {
		return core.Tool{}, err
	}
	run := func(_ context.Context, in json.RawMessage) (json.RawMessage, core.Usage, error) {
		var args updateArgs
		if err := json.Unmarshal(in, &args); err != nil {
			return nil, core.Usage{}, err
		}
		j.mu.Lock()
		defer j.mu.Unlock()
		for i, t := range j.tasks {
			if t.ID == args.TaskID {
				j.tasks[i].Status = args.Status
				out, mErr := json.Marshal(updateResult{Message: "Task updated: " + args.TaskID})
				return out, core.Usage{}, mErr
			}
		}
		out, mErr := json.Marshal(updateResult{Message: "Task not found: " + args.TaskID})
		return out, core.Usage{}, mErr
	}
	return core.Tool{
		Name:        "journal_update_task",
		Description: "Updates the status of an existing task in the checklist.",
		IOSchema:    schema,
		Run:         run,
	}, nil
}
