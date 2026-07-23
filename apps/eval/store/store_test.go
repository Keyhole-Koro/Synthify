package store

import (
	"testing"

	"github.com/synthify/backend/apps/eval/runner"
	"github.com/stretchr/testify/require"
)

func TestSummarize(t *testing.T) {
	t.Parallel()

	summary := Summarize([]runner.Result{
		{Passed: true, Model: "gemini-2.5-flash", InputTokens: 100, OutputTokens: 20},
		{Passed: false, Model: "gemini-2.5-flash", InputTokens: 80, OutputTokens: 10},
		{Passed: true, Model: "gemini-2.5-flash", InputTokens: 120, OutputTokens: 30},
	})

	require.Equal(t, int64(3), summary.CaseCount)
	require.Equal(t, int64(2), summary.PassedCount)
	require.Equal(t, int64(1), summary.FailedCount)
	require.InDelta(t, 2.0/3.0, summary.PassRate, 0.0001)
	require.Equal(t, "failed", summary.Status)
	require.Equal(t, "gemini-2.5-flash", summary.Model)
	require.Equal(t, int64(300), summary.InputTokens)
	require.Equal(t, int64(60), summary.OutputTokens)
}

func TestSummarizeMixedModels(t *testing.T) {
	t.Parallel()

	summary := Summarize([]runner.Result{
		{Passed: true, Model: "gemini-2.5-flash"},
		{Passed: true, Model: "gemini-2.5-pro"},
	})

	require.Equal(t, "mixed", summary.Model)
	require.Equal(t, "succeeded", summary.Status)
	require.Equal(t, 1.0, summary.PassRate)
}
