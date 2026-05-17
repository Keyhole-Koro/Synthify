package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/synthify/backend/apps/eval/report"
	"github.com/synthify/backend/apps/eval/runner"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/packages/shared/config"
)

func main() {
	casePath := flag.String("case", "", "YAML eval case path")
	format := flag.String("format", "table", "output format: table or json")
	timeout := flag.Duration("timeout", 60*time.Second, "eval timeout")
	flag.Parse()

	if *casePath == "" {
		fmt.Fprintln(os.Stderr, "--case is required")
		os.Exit(2)
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

	res, err := runner.Runner{LLM: client}.RunCaseFile(ctx, *casePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run eval: %v\n", err)
		os.Exit(1)
	}

	if err := report.Write(os.Stdout, *format, []runner.Result{res}); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
	if !res.Passed {
		os.Exit(1)
	}
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
