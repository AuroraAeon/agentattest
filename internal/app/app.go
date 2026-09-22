package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"agentattest.dev/agentattest/internal/cache"
	"agentattest.dev/agentattest/internal/contracts"
	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/predicate"
	"agentattest.dev/agentattest/internal/verify"
)

func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	ctx := context.Background()
	var err error
	switch args[0] {
	case "init":
		err = runInit(ctx, args[1:], stdout)
	case "digest":
		err = runDigest(ctx, args[1:], stdout)
	case "context":
		err = runContext(ctx, args[1:], stdout)
	case "predicate":
		err = runPredicate(ctx, args[1:], stdout)
	case "verify":
		return runVerify(ctx, args[1:], stdout, stderr)
	case "summary":
		err = runSummary(ctx, args[1:], stdout)
	default:
		usage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

func runInit(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dir := fs.String("dir", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	dbPath, err := cache.Init(ctx, *dir)
	if err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"created": true,
		"dir":     filepath.ToSlash(filepath.Join(*dir, cache.DirName)),
		"db":      filepath.ToSlash(dbPath),
		"config":  filepath.ToSlash(filepath.Join(*dir, cache.DirName, cache.ConfigName)),
	})
}

func runDigest(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("digest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := gitbind.Compute(ctx, *repo)
	if err != nil {
		return err
	}
	_ = cache.IndexDigest(ctx, *repo, result)
	return writeJSON(stdout, result)
}

func runPredicate(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "create" {
		return errors.New("usage: agentattest predicate create [--repo DIR] [--out PATH] [--version v0|v1]")
	}
	fs := flag.NewFlagSet("predicate create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository directory")
	out := fs.String("out", "", "output statement path")
	version := fs.String("version", "v0", "predicate contract version: v0 or v1")
	repoURL := fs.String("repo-url", "", "override repository URL")
	baseCommit := fs.String("base-commit", "", "override base commit")
	agentName := fs.String("agent-name", "", "agent name")
	agentVersion := fs.String("agent-version", "", "agent version")
	captureMethod := fs.String("capture-method", "", "v1: how the run was captured (wrapper|ci-step|manual)")
	captureHarness := fs.String("capture-harness", "", "v1: harness name when capture-method is harness-native")
	agentConfig := fs.String("agent-config", "", "v1: path to the operating contract file (e.g. AGENTS.md) to bind by digest")
	modelProvider := fs.String("model-provider", "", "v1: model provider (with --model-id)")
	modelID := fs.String("model-id", "", "v1: model id (with --model-provider)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	digest, err := gitbind.Compute(ctx, *repo)
	if err != nil {
		return err
	}

	opts := predicate.CreateOptions{
		RepoURL:      *repoURL,
		BaseCommit:   *baseCommit,
		AgentName:    *agentName,
		AgentVersion: *agentVersion,
	}

	var stmt map[string]any
	switch *version {
	case "", "v0":
		if *captureMethod != "" || *captureHarness != "" || *agentConfig != "" || *modelProvider != "" || *modelID != "" {
			return errors.New("--capture-method/--capture-harness/--agent-config/--model-provider/--model-id require --version v1")
		}
		pred := predicate.Create(digest, opts)
		stmt = predicate.StatementFor(digest, pred)
	case "v1":
		if err := applyV1PredicateOptions(&opts, *captureMethod, *captureHarness, *agentConfig, *modelProvider, *modelID); err != nil {
			return err
		}
		pred := predicate.CreateV1(digest, opts)
		stmt = predicate.StatementForV1(digest, pred)
	default:
		return fmt.Errorf("unsupported --version %q (want v0 or v1)", *version)
	}

	if *out != "" {
		content, err := json.MarshalIndent(stmt, "", "  ")
		if err != nil {
			return err
		}
		content = append(content, '\n')
		return os.WriteFile(*out, content, 0o644)
	}
	return writeJSON(stdout, stmt)
}

// applyV1PredicateOptions validates and folds v1-only flags into opts. It
// refuses harness-native capture because that requires a bound OTel trace that
// only a harness capture integration (not this CLI) can produce.
func applyV1PredicateOptions(opts *predicate.CreateOptions, captureMethod, captureHarness, agentConfig, modelProvider, modelID string) error {
	switch captureMethod {
	case "", "wrapper", "ci-step", "manual":
		opts.CaptureMethod = captureMethod
	case "harness-native":
		return errors.New("--capture-method harness-native requires a bound OTel trace from a harness capture integration; use wrapper (default), ci-step, or manual")
	default:
		return fmt.Errorf("unsupported --capture-method %q (want wrapper, ci-step, or manual)", captureMethod)
	}
	opts.CaptureHarness = captureHarness

	if agentConfig != "" {
		data, err := os.ReadFile(agentConfig)
		if err != nil {
			return fmt.Errorf("read --agent-config %q: %w", agentConfig, err)
		}
		sum := sha256.Sum256(data)
		opts.AgentConfigPath = filepath.Base(agentConfig)
		opts.AgentConfigSHA256 = hex.EncodeToString(sum[:])
	}

	if (modelProvider == "") != (modelID == "") {
		return errors.New("--model-provider and --model-id must be supplied together")
	}
	opts.ModelProvider = modelProvider
	opts.ModelID = modelID
	return nil
}

func runVerify(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "predicate" {
		fmt.Fprintln(stderr, "error: usage: agentattest verify predicate --statement PATH --context PATH")
		return 2
	}
	fs := flag.NewFlagSet("verify predicate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statementPath := fs.String("statement", "", "statement JSON path")
	contextPath := fs.String("context", "", "verifier context JSON path")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	if *statementPath == "" || *contextPath == "" {
		fmt.Fprintln(stderr, "error: --statement and --context are required")
		return 2
	}

	statementJSON, err := os.ReadFile(*statementPath)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	contextJSON, err := os.ReadFile(*contextPath)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	files, err := contracts.Locate("")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	result := verify.Predicate(ctx, statementJSON, contextJSON, verify.Options{Contracts: files})
	if err := writeJSON(stdout, result); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if !result.Valid {
		return 1
	}
	return 0
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  agentattest init [--dir DIR]")
	fmt.Fprintln(w, "  agentattest digest [--repo DIR]")
	fmt.Fprintln(w, "  agentattest context [--repo DIR] [--required-level evidence-grade|policy-grade|high-assurance]")
	fmt.Fprintln(w, "  agentattest predicate create [--repo DIR] [--out PATH] [--version v0|v1]")
	fmt.Fprintln(w, "      v1 adds: --capture-method wrapper|ci-step|manual [--capture-harness NAME]")
	fmt.Fprintln(w, "               [--agent-config PATH] [--model-provider P --model-id M]")
	fmt.Fprintln(w, "  agentattest verify predicate --statement PATH --context PATH")
	fmt.Fprintln(w, "  agentattest summary --statement PATH --context PATH")
}
