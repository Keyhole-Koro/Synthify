package agents

import (
	"encoding/json"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
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
			if err := o.base.IncrementToolRuns(ctx); err != nil {
				return nil, err
			}

			stage := stageTools[t.Name()]
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
				o.base.Logger.Warn(ctx, "orchestrator.checkpoint_version_mismatch", nil, map[string]any{"stage": stage, "version": envelope.SchemaVersion, "expected": currentCheckpointVersion})
				_ = o.repo.UpsertStageRunning(ctx, jobID, stage)
				return nil, nil
			}
			if o.base.Job != nil && envelope.DocumentID != o.base.Job.DocumentID {
				o.base.Logger.Warn(ctx, "orchestrator.checkpoint_document_id_mismatch", nil, map[string]any{"stage": stage, "doc_id": envelope.DocumentID, "expected": o.base.Job.DocumentID})
				_ = o.repo.UpsertStageRunning(ctx, jobID, stage)
				return nil, nil
			}
			o.base.Logger.Info(ctx, "orchestrator.resuming_from_checkpoint", map[string]any{"stage": stage})
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

			stage := stageTools[t.Name()]
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
					o.base.Logger.Error(ctx, "orchestrator.write_checkpoint_failed", writeErr, map[string]any{"stage": stage})
				}
			} else if err != nil && stage != "" && jobID != "" && o.repo != nil {
				_ = o.repo.MarkStageFailed(ctx, jobID, stage, err.Error())
			}

			return result, err
		},
	}
}
