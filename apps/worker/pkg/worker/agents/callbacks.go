package agents

import (
	"encoding/json"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

func (o *Orchestrator) beforeModelCallbacks() []llmagent.BeforeModelCallback {
	return []llmagent.BeforeModelCallback{
		func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
			if err := o.base.IncrementLLMCalls(ctx); err != nil {
				return nil, err
			}
			workingMemory := o.base.RenderWorkingMemory()
			if req.Config == nil {
				req.Config = &genai.GenerateContentConfig{}
			}
			if req.Config.SystemInstruction == nil {
				req.Config.SystemInstruction = genai.NewContentFromText(workingMemory, "system")
			} else {
				existing := ""
				for _, part := range req.Config.SystemInstruction.Parts {
					existing += part.Text
				}
				req.Config.SystemInstruction = genai.NewContentFromText(existing+"\n\n"+workingMemory, "system")
			}
			return nil, nil
		},
	}
}

func (o *Orchestrator) beforeToolCallbacks() []llmagent.BeforeToolCallback {
	return []llmagent.BeforeToolCallback{
		func(ctx tool.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
			// Loop guard: if the agent keeps re-issuing the same tool call with
			// the same args, short-circuit with a nudge instead of running the
			// tool again. Returning a non-nil map makes ADK skip the tool and use
			// this map as the result, so the agent sees feedback rather than an
			// error and can change course. Runs before the budget increments so a
			// throttled call does not consume the tool-run quota.
			if guard := o.repeatGuard.Load(); guard != nil {
				if nudge, count := guard.observe(t.Name(), args); nudge {
					o.base.Logger.Warn("orchestrator.repeated_tool_call",
						"tool", t.Name(), "consecutive_calls", count)
					return map[string]any{
						"error": "repeated_call_detected",
						"message": "You have called this tool with the same arguments " +
							"several times in a row without making progress. Do not call it " +
							"again the same way. Either change the arguments, use a different " +
							"tool, or proceed to the next step of your workflow with the " +
							"information you already have.",
					}, nil
				}
			}

			if err := o.base.IncrementToolRuns(ctx); err != nil {
				return nil, err
			}
			// Dynamic (transform-backed) tools also count against the separate
			// MaxTransformRuns cap so transform execution is bounded
			// independently of builtin tool runs.
			if o.isDynamicTool(t.Name()) {
				if err := o.base.IncrementTransformRuns(ctx); err != nil {
					return nil, err
				}
			}

			// stage doubles as the checkpoint key; for per-item tools
			// (generate_html_summary) it carries an item suffix so each item
			// resumes independently.
			stage := checkpointKey(t.Name(), args)
			if stage == "" || o.repo == nil || o.fs == nil {
				return nil, nil
			}

			jobIDPtr := o.currentJobID.Load()
			if jobIDPtr == nil || *jobIDPtr == "" {
				return nil, nil
			}
			jobID := *jobIDPtr

			var envelope domain.CheckpointEnvelope
			found, err := o.fs.ReadCheckpoint(jobID, stage, &envelope)
			if err != nil || !found {
				_ = o.repo.UpsertStageRunning(ctx, jobID, stage)
				return nil, nil
			}
			if envelope.SchemaVersion != currentCheckpointVersion {
				o.base.Logger.Warn("orchestrator.checkpoint_version_mismatch", "stage", stage, "version", envelope.SchemaVersion, "expected", currentCheckpointVersion)
				_ = o.repo.UpsertStageRunning(ctx, jobID, stage)
				return nil, nil
			}
			if o.base.Job != nil && envelope.DocumentID != o.base.Job.DocumentID {
				o.base.Logger.Warn("orchestrator.checkpoint_document_id_mismatch", "stage", stage, "doc_id", envelope.DocumentID, "expected", o.base.Job.DocumentID)
				_ = o.repo.UpsertStageRunning(ctx, jobID, stage)
				return nil, nil
			}
			o.base.Logger.Info("orchestrator.resuming_from_checkpoint", "stage", stage)
			return envelope.Outputs, nil
		},
	}
}

func (o *Orchestrator) afterToolCallbacks() []llmagent.AfterToolCallback {
	return []llmagent.AfterToolCallback{
		func(ctx tool.Context, t tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
			start := time.Now()
			argJSON, _ := json.Marshal(args)
			resJSON, _ := json.Marshal(result)
			if err != nil {
				resJSON, _ = json.Marshal(map[string]string{"error": err.Error()})
			}

			jobIDPtr := o.currentJobID.Load()
			jobID := ""
			if jobIDPtr != nil {
				jobID = *jobIDPtr
			}

			if o.repo != nil && jobID != "" {
				_ = o.repo.LogToolCall(ctx, jobID, t.Name(), string(argJSON), string(resJSON), time.Since(start).Milliseconds())
			}

			// Same key function as the read path so the recorded checkpoint is
			// the one a resume will look for (per-item suffix for html_summary).
			stage := checkpointKey(t.Name(), args)
			if err == nil && stage != "" && jobID != "" && o.repo != nil && o.fs != nil {
				docID := ""
				wsID := ""
				if o.base.Job != nil {
					docID = o.base.Job.DocumentID
					wsID = o.base.Job.WorkspaceID
				}
				envelope := domain.CheckpointEnvelope{
					SchemaVersion: currentCheckpointVersion,
					Kind:          "synthify.worker_checkpoint",
					Stage:         stage,
					JobID:         jobID,
					DocumentID:    docID,
					WorkspaceID:   wsID,
					CreatedAt:     time.Now().UTC().Format(time.RFC3339),
					Inputs:        args,
					Outputs:       result,
				}
				if writeErr := o.fs.WriteCheckpoint(jobID, stage, envelope); writeErr == nil {
					_ = o.repo.MarkStageSucceeded(ctx, jobID, stage, o.fs.CheckpointPath(jobID, stage))
				} else {
					o.base.Logger.Error("orchestrator.write_checkpoint_failed", "error", writeErr.Error(), "stage", stage)
				}
			} else if err != nil && stage != "" && jobID != "" && o.repo != nil {
				_ = o.repo.MarkStageFailed(ctx, jobID, stage, err.Error())
			}

			return result, err
		},
	}
}
