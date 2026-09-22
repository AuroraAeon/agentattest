package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/predicate"
	"agentattest.dev/agentattest/internal/verify"
)

// TestPredicateCreateV1VerifiesEvidenceGrade proves that a v1 statement produced
// by the same code path the CLI uses round-trips through the real verifier
// pipeline (schema + CUE + Rego) as a valid evidence-grade claim.
func TestPredicateCreateV1VerifiesEvidenceGrade(t *testing.T) {
	result := gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		HeadCommit: "89abcdef0123456789abcdef0123456789abcdef",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}

	pred := predicate.CreateV1(result, predicate.CreateOptions{
		AgentName:     "Claude Code",
		AgentVersion:  "1.2.3",
		Now:           time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC),
		CaptureMethod: "wrapper",
		ModelProvider: "anthropic",
		ModelID:       "claude-sonnet-4",
	})
	stmt := predicate.StatementForV1(result, pred)

	if stmt["predicateType"] != "https://agentattest.dev/predicate/v1" {
		t.Fatalf("predicateType = %v", stmt["predicateType"])
	}

	stmtJSON, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	contextInput := map[string]any{
		"repoUrl":       result.RepoURL,
		"baseCommit":    result.BaseCommit,
		"requiredLevel": "evidence-grade",
		"subjects": []map[string]any{
			{"name": "patch.diff", "algorithm": "sha256", "digest": result.Patch.Digest},
		},
	}
	contextJSON, err := json.Marshal(contextInput)
	if err != nil {
		t.Fatal(err)
	}

	got := verify.Predicate(context.Background(), stmtJSON, contextJSON, verify.Options{})
	if !got.Valid {
		t.Fatalf("expected valid v1 evidence-grade, got %+v", got)
	}
	if got.Level != "evidence-grade" {
		t.Fatalf("level = %q, want evidence-grade", got.Level)
	}
}
