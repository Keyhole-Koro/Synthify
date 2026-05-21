package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/synthify/backend/apps/eval/artifact"
	"github.com/synthify/backend/apps/eval/report"
	"github.com/synthify/backend/apps/eval/runner"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/packages/shared/config"
)

func main() {
	casePath := flag.String("case", "", "YAML eval case path")
	casesPath := flag.String("cases", "", "YAML eval case directory")
	format := flag.String("format", "table", "output format: table or json")
	outPath := flag.String("out", "", "optional path to write the report")
	outGCS := flag.String("out-gcs", "", "optional gs:// prefix to write the report artifact")
	variant := flag.String("variant", "", "variant name under apps/eval/variants/{name}; empty = production prompt")
	variantsDir := flag.String("variants-dir", "apps/eval/variants", "root directory holding variant prompt sets")
	timeout := flag.Duration("timeout", 60*time.Second, "eval timeout")
	flag.Parse()

	if (*casePath == "") == (*casesPath == "") {
		fmt.Fprintln(os.Stderr, "set exactly one of --case or --cases")
		os.Exit(2)
	}

	var renderer *prompts.Renderer
	promptSource := "production"
	if *variant != "" {
		dir := filepath.Join(*variantsDir, *variant)
		r, err := prompts.FromDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load variant %q: %v\n", *variant, err)
			os.Exit(2)
		}
		renderer = r
		promptSource = "variant:" + *variant
	}

	if err := loadDotEnv(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "load .env: %v\n", err)
		os.Exit(2)
	}

	cfg := config.LoadLLM()
	if !cfg.Enabled() {
		fmt.Fprintln(os.Stderr, "GEMINI_API_KEY or GOOGLE_API_KEY is required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := llm.NewGeminiClient(ctx, cfg, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init llm client: %v\n", err)
		os.Exit(1)
	}

	target := *casePath
	if target == "" {
		target = *casesPath
	}
	caseFiles, err := runner.CaseFiles(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load cases: %v\n", err)
		os.Exit(1)
	}

	results, err := runner.Runner{
		LLM:          client,
		Renderer:     renderer,
		PromptSource: promptSource,
	}.RunCaseFiles(ctx, caseFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run eval: %v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	if err := report.Write(&buf, *format, results); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "write stdout: %v\n", err)
		os.Exit(1)
	}
	if *outPath != "" {
		if err := writeReportFile(*outPath, buf.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "write --out: %v\n", err)
			os.Exit(1)
		}
	}
	if gcsURI := resolveGCSOutputURI(*outGCS); gcsURI != "" {
		res, err := artifact.UploadGCS(ctx, artifact.GCSConfig{PrefixURI: gcsURI, PromptSource: promptSource}, buf.Bytes())
		if err != nil {
			fmt.Fprintf(os.Stderr, "write --out-gcs: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "eval artifact uploaded: %s\n", res.URI)
	}
	if !allPassed(results) {
		os.Exit(1)
	}
}

func resolveGCSOutputURI(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return strings.TrimSpace(flagValue)
	}
	return strings.TrimSpace(os.Getenv("EVAL_OUTPUT_GCS_URI"))
}

func allPassed(results []runner.Result) bool {
	for _, res := range results {
		if !res.Passed {
			return false
		}
	}
	return true
}

func writeReportFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, b, 0o644)
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
