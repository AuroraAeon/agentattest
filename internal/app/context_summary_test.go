package app

import (
	"strings"
	"testing"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/verify"
)

func TestBuildVerifierContext(t *testing.T) {
	result := gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: strings.Repeat("a", 64)},
	}

	ctx := buildVerifierContext(result, "")
	if ctx["repoUrl"] != result.RepoURL {
		t.Fatalf("repoUrl = %v", ctx["repoUrl"])
	}
	if ctx["baseCommit"] != result.BaseCommit {
		t.Fatalf("baseCommit = %v", ctx["baseCommit"])
	}
	if ctx["requiredLevel"] != "evidence-grade" {
		t.Fatalf("default requiredLevel = %v, want evidence-grade", ctx["requiredLevel"])
	}
	subjects, ok := ctx["subjects"].([]map[string]any)
	if !ok || len(subjects) != 1 {
		t.Fatalf("subjects = %v", ctx["subjects"])
	}
	if subjects[0]["name"] != "patch.diff" || subjects[0]["algorithm"] != "sha256" || subjects[0]["digest"] != result.Patch.Digest {
		t.Fatalf("subject = %v", subjects[0])
	}

	if got := buildVerifierContext(result, "policy-grade")["requiredLevel"]; got != "policy-grade" {
		t.Fatalf("requiredLevel = %v, want policy-grade", got)
	}
}

func sampleStatement() map[string]any {
	return map[string]any{
		"subject": []any{
			map[string]any{"name": "patch.diff", "digest": map[string]any{"sha256": strings.Repeat("a", 64)}},
			map[string]any{"name": "repo-tree", "digest": map[string]any{"sha256": strings.Repeat("b", 64)}},
		},
		"predicate": map[string]any{
			"repo": map[string]any{
				"url":        "https://github.com/example/agentattest",
				"baseCommit": "0123456789abcdef0123456789abcdef01234567",
			},
			"privacy": map[string]any{
				"redactionPolicy": "default-minimal-v1",
				"rawPrompt":       map[string]any{"stored": false},
				"rawToolOutputs":  map[string]any{"stored": false},
			},
			"capture": map[string]any{"method": "harness-native", "harness": "claude-code"},
		},
	}
}

func sampleContext() map[string]any {
	return map[string]any{
		"repoUrl":           "https://github.com/example/agentattest",
		"baseCommit":        "0123456789abcdef0123456789abcdef01234567",
		"verifiedSigner":    "sigstore:github:example-agent",
		"verifiedBuilderId": "https://github.com/example/agentattest/.github/workflows/w.yml@refs/heads/main",
		"verifiedIssuer":    "https://token.actions.githubusercontent.com",
	}
}

func TestRenderSummaryValidPolicyGrade(t *testing.T) {
	result := verify.Result{Valid: true, Level: "policy-grade"}
	out := renderSummary(result, sampleStatement(), sampleContext())

	for _, want := range []string{
		"## Agent Provenance: valid (policy-grade)",
		"patch.diff",
		"repo-tree",
		"repo URL matches context: yes",
		"matches context: yes",
		"sigstore:github:example-agent",
		"redaction policy: default-minimal-v1",
		"raw prompt stored: no",
		"raw tool output stored: no",
		"**Capture**: harness-native (claude-code)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Failure codes") {
		t.Fatalf("valid summary must not list failure codes:\n%s", out)
	}
}

func TestRenderSummaryInvalidListsCodes(t *testing.T) {
	result := verify.Result{Valid: false, Level: "policy-grade", FailureCodes: []string{"subject_digest_mismatch", "base_commit_mismatch"}}
	out := renderSummary(result, sampleStatement(), sampleContext())
	if !strings.Contains(out, "invalid") {
		t.Fatalf("expected invalid status:\n%s", out)
	}
	if !strings.Contains(out, "subject_digest_mismatch") || !strings.Contains(out, "base_commit_mismatch") {
		t.Fatalf("expected failure codes listed:\n%s", out)
	}
}

func TestRenderSummaryIsDeterministic(t *testing.T) {
	result := verify.Result{Valid: true, Level: "policy-grade"}
	first := renderSummary(result, sampleStatement(), sampleContext())
	second := renderSummary(result, sampleStatement(), sampleContext())
	if first != second {
		t.Fatalf("summary is not deterministic:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
