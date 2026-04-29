package app

import (
	"context"
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
	case "predicate":
		err = runPredicate(ctx, args[1:], stdout)
	case "verify":
		return runVerify(ctx, args[1:], stdout, stderr)
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
		return errors.New("usage: agentattest predicate create [--repo DIR] [--out PATH]")
	}
	fs := flag.NewFlagSet("predicate create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repo := fs.String("repo", ".", "repository directory")
	out := fs.String("out", "", "output statement path")
	repoURL := fs.String("repo-url", "", "override repository URL")
	baseCommit := fs.String("base-commit", "", "override base commit")
	agentName := fs.String("agent-name", "", "agent name")
	agentVersion := fs.String("agent-version", "", "agent version")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	digest, err := gitbind.Compute(ctx, *repo)
	if err != nil {
		return err
	}
	pred := predicate.Create(digest, predicate.CreateOptions{
		RepoURL:      *repoURL,
		BaseCommit:   *baseCommit,
		AgentName:    *agentName,
		AgentVersion: *agentVersion,
	})
	stmt := predicate.StatementFor(digest, pred)

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
	files, err := contracts.Locate(*statementPath)
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
	fmt.Fprintln(w, "  agentattest predicate create [--repo DIR] [--out PATH]")
	fmt.Fprintln(w, "  agentattest verify predicate --statement PATH --context PATH")
}
