package predicate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
)

func TestCreateDefaultsDoNotStoreRawPromptOrToolOutputs(t *testing.T) {
	result := gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		HeadCommit: "89abcdef0123456789abcdef0123456789abcdef",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	pred := Create(result, CreateOptions{Now: time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC)})
	privacy := pred["privacy"].(map[string]any)
	rawPrompt := privacy["rawPrompt"].(map[string]any)
	rawToolOutputs := privacy["rawToolOutputs"].(map[string]any)

	if rawPrompt["stored"] != false || rawPrompt["visibility"] != "none" {
		t.Fatalf("rawPrompt = %v", rawPrompt)
	}
	if rawToolOutputs["stored"] != false || rawToolOutputs["visibility"] != "none" {
		t.Fatalf("rawToolOutputs = %v", rawToolOutputs)
	}
}

func TestCreateV1Shape(t *testing.T) {
	result := gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(cfgPath, []byte("build: go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	wantSum := hex.EncodeToString(func() []byte { s := sha256.Sum256(data); return s[:] }())

	pred := CreateV1(result, CreateOptions{
		AgentName:         "Claude Code",
		AgentVersion:      "1.2.3",
		Now:               time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC),
		CaptureMethod:     "ci-step",
		AgentConfigPath:   "AGENTS.md",
		AgentConfigSHA256: wantSum,
		ModelProvider:     "anthropic",
		ModelID:           "claude-sonnet-4",
	})

	if pred["predicateVersion"] != "v1" {
		t.Fatalf("predicateVersion = %v", pred["predicateVersion"])
	}
	capture := pred["capture"].(map[string]any)
	if capture["method"] != "ci-step" {
		t.Fatalf("capture.method = %v", capture["method"])
	}
	cfg := pred["agentConfig"].(map[string]any)
	if cfg["primaryFile"] != "AGENTS.md" {
		t.Fatalf("agentConfig.primaryFile = %v", cfg["primaryFile"])
	}
	if got := cfg["primaryDigest"].(map[string]any)["sha256"]; got != wantSum {
		t.Fatalf("agentConfig.primaryDigest = %v, want %v", got, wantSum)
	}
	model := pred["model"].(map[string]any)
	if model["provider"] != "anthropic" || model["modelId"] != "claude-sonnet-4" {
		t.Fatalf("model = %v", model)
	}

	// v0 must never carry v1-only keys.
	v0 := Create(result, CreateOptions{Now: time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC)})
	if v0["predicateVersion"] != "v0" {
		t.Fatalf("v0 predicateVersion = %v", v0["predicateVersion"])
	}
	for _, key := range []string{"capture", "agentConfig", "model"} {
		if _, ok := v0[key]; ok {
			t.Fatalf("v0 predicate must not contain %q", key)
		}
	}

	if StatementForV1(result, pred)["predicateType"] != "https://agentattest.dev/predicate/v1" {
		t.Fatal("StatementForV1 predicateType mismatch")
	}
	if StatementFor(result, v0)["predicateType"] != "https://agentattest.dev/predicate/v0" {
		t.Fatal("StatementFor predicateType mismatch")
	}
}
