package llm

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	connect "connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	localproviderv1 "github.com/synthify/backend/internal/gen/synthify/localprovider/v1"
	localproviderv1connect "github.com/synthify/backend/internal/gen/synthify/localprovider/v1/localproviderv1connect"
)

func TestLocalProviderCrossLanguageContract(t *testing.T) {
	python := os.Getenv("SYNTHIFY_LOCAL_PROVIDER_PYTHON")
	if python == "" {
		t.Skip("set SYNTHIFY_LOCAL_PROVIDER_PYTHON to run the Python provider contract gate")
	}

	repoRoot := findRepoRoot(t)
	tokenFile := filepath.Join(t.TempDir(), "provider-token")
	require.NoError(t, os.WriteFile(tokenFile, []byte(localProviderTestToken+"\n"), 0o600))
	endpoint, events := startPythonLocalProvider(t, python, repoRoot, tokenFile)

	raw := localproviderv1connect.NewLocalProviderServiceClient(http.DefaultClient, endpoint)
	_, err := raw.Check(context.Background(), connect.NewRequest(&localproviderv1.CheckRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	invalid := connect.NewRequest(&localproviderv1.GenerateTextRequest{})
	invalid.Header().Set("Authorization", "Bearer "+localProviderTestToken)
	_, err = raw.GenerateText(context.Background(), invalid)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	cfg := config.LLM{
		Provider:                    config.LLMProviderAntigravity,
		RuntimeEnv:                  "test",
		DeploymentMode:              "self-hosted",
		LocalProviderEndpoint:       endpoint,
		LocalProviderTokenFile:      tokenFile,
		LocalProviderRequestTimeout: 5 * time.Second,
		LocalProviderCancelTimeout:  2 * time.Second,
		LocalProviderHealthTimeout:  2 * time.Second,
	}
	client, err := NewLocalProviderClient(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.Equal(t, "cross-language-test", client.Capabilities().ServerVersion)

	client.newGenerationID = func() string { return "generation-cross-text" }
	text, usage, err := client.GenerateText(context.Background(), TextRequest{UserPrompt: "hello"})
	require.NoError(t, err)
	require.Equal(t, "text:hello", text)
	require.Equal(t, Usage{Model: "antigravity:test-model", InputTokens: 3, OutputTokens: 2}, usage)

	type structuredOutput struct {
		Answer string `json:"answer"`
	}
	client.newGenerationID = func() string { return "generation-cross-structured" }
	payload, usage, err := client.GenerateStructured(context.Background(), StructuredRequest{
		UserPrompt: "structured",
		Schema:     structuredOutput{},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"answer":"ok"}`, string(payload))
	require.Equal(t, Usage{Model: "antigravity:test-model", InputTokens: 5, OutputTokens: 3}, usage)

	client.newGenerationID = func() string { return "generation-cross-rate-limit" }
	_, _, err = client.GenerateText(context.Background(), TextRequest{UserPrompt: "rate-limit"})
	var providerErr *LocalProviderError
	require.ErrorAs(t, err, &providerErr)
	require.Equal(t, connect.CodeResourceExhausted, providerErr.Code)
	require.Equal(t, localproviderv1.LocalProviderErrorDetail_REASON_RATE_LIMITED, providerErr.Reason)
	require.False(t, providerErr.TurnStarted)
	require.Equal(t, 250*time.Millisecond, providerErr.RetryAfter)
	require.True(t, providerErr.RetryablePreTurnRateLimit())

	client.newGenerationID = func() string { return "generation-cross-cancel" }
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, _, callErr := client.GenerateText(ctx, TextRequest{UserPrompt: "wait-for-cancel"})
		result <- callErr
	}()
	waitForProviderEvent(t, events, "STARTED generation-cross-cancel")
	cancel()
	select {
	case callErr := <-result:
		require.ErrorIs(t, callErr, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the cancelled generation to return")
	}
	waitForProviderEvent(t, events, "CANCELLED generation-cross-cancel")
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(directory, "go.mod")); statErr == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("could not find repository root")
		}
		directory = parent
	}
}

func startPythonLocalProvider(t *testing.T, python, repoRoot, tokenFile string) (string, <-chan string) {
	t.Helper()
	script := filepath.Join(repoRoot, "apps/local-provider/tests/fake_server.py")
	command := exec.Command(python, script, "--token-file", tokenFile)
	command.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(repoRoot, "apps/local-provider/src"))
	stdout, err := command.StdoutPipe()
	require.NoError(t, err)
	var stderr lockedStringBuilder
	command.Stderr = &stderr
	require.NoError(t, command.Start())

	var waitErr error
	done := make(chan struct{})
	go func() {
		waitErr = command.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("Python local provider did not exit; stderr: %s", stderr.String())
		}
	})

	lines := make(chan string, 16)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()

	select {
	case line := <-lines:
		var port int
		_, scanErr := fmt.Sscanf(line, "READY %d", &port)
		require.NoErrorf(t, scanErr, "unexpected Python provider output %q; stderr: %s", line, stderr.String())
		return fmt.Sprintf("http://127.0.0.1:%d", port), lines
	case <-done:
		t.Fatalf("Python local provider exited before readiness: %v; stderr: %s", waitErr, stderr.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for Python local provider; stderr: %s", stderr.String())
	}
	return "", nil
}

func waitForProviderEvent(t *testing.T, events <-chan string, expected string) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("Python local provider exited while waiting for %q", expected)
			}
			if event == expected {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for Python provider event %q", expected)
		}
	}
}

type lockedStringBuilder struct {
	mu      sync.Mutex
	builder strings.Builder
}

func (b *lockedStringBuilder) Write(contents []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.builder.Write(contents)
}

func (b *lockedStringBuilder) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.builder.String()
}
