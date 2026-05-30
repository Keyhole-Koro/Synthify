package agents

import (
	"encoding/json"
	"log/slog"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
)

// NewPayloadLoggingCallback returns an ADK after-model callback that logs the
// raw model response of every agent turn. It exists because LOG_LLM_PAYLOAD on
// the llm.GeminiClient only covers the direct text-generation path; the
// orchestrator drives the model through ADK, so without this the agent's
// per-turn responses — and crucially whether the model emitted a structured
// functionCall or just prose — are never observable. That distinction is the
// difference between "tool dispatched" and "spun until timeout with zero tool
// runs", so it is the first thing to check when diagnosing a stuck job.
//
// It only logs when enabled is true (wired from LOG_LLM_PAYLOAD) to avoid
// spilling full prompt/response text into logs by default. Partial streaming
// events are skipped so each turn is logged once, on its final response.
func NewPayloadLoggingCallback(enabled bool, logger *slog.Logger) llmagent.AfterModelCallback {
	return func(ctx agent.CallbackContext, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
		if !enabled || logger == nil {
			return nil, nil
		}
		if respErr != nil {
			logger.Error("agent.llm.response_error",
				"error", respErr.Error(),
				"model", responseModel(resp))
			return nil, nil
		}
		if resp == nil || resp.Partial {
			return nil, nil
		}

		var text string
		var funcCalls []string
		if resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part == nil {
					continue
				}
				if part.Text != "" {
					text += part.Text
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					funcCalls = append(funcCalls, part.FunctionCall.Name+"("+string(args)+")")
				}
			}
		}

		logger.Info("agent.llm.response",
			"model", resp.ModelVersion,
			"finish_reason", string(resp.FinishReason),
			"function_calls", funcCalls,
			"function_call_count", len(funcCalls),
			"text_len", len(text),
			"text", text,
		)
		return nil, nil
	}
}

func responseModel(resp *model.LLMResponse) string {
	if resp == nil {
		return ""
	}
	return resp.ModelVersion
}
