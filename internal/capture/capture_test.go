package capture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/verify"
)

func sampleResult() gitbind.Result {
	return gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: strings.Repeat("a", 64)},
	}
}

func baseMeta() RunMetadata {
	return RunMetadata{
		AgentName:      "claude-code",
		AgentVersion:   "1.2.3",
		InvocationKind: "local-cli",
		ModelProvider:  "anthropic",
		ModelID:        "claude-sonnet-4",
		TraceFormat:    "otlp-json",
		TraceIDs:       []string{"4bf92f3577b34da6a3ce929d0e0e4736"},
		TraceURI:       "artifact://agentattest/traces/run.otel.json",
		TraceDigest:    strings.Repeat("c", 64),
		Now:            time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC),
	}
}

func verifyEvidenceGrade(t *testing.T, stmt map[string]any) verify.Result {
	t.Helper()
	result := sampleResult()
	stmtJSON, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	ctxInput := map[string]any{
		"repoUrl":       result.RepoURL,
		"baseCommit":    result.BaseCommit,
		"requiredLevel": "evidence-grade",
		"subjects": []map[string]any{
			{"name": "patch.diff", "algorithm": "sha256", "digest": result.Patch.Digest},
		},
	}
	ctxJSON, err := json.Marshal(ctxInput)
	if err != nil {
		t.Fatal(err)
	}
	return verify.Predicate(context.Background(), stmtJSON, ctxJSON, verify.Options{})
}

func TestCaptureStatementVerifiesHarnessNative(t *testing.T) {
	stmt, err := Statement(sampleResult(), baseMeta())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if stmt["predicateType"] != "https://agentattest.dev/predicate/v1" {
		t.Fatalf("predicateType = %v", stmt["predicateType"])
	}
	pred := stmt["predicate"].(map[string]any)
	capture := pred["capture"].(map[string]any)
	if capture["method"] != "harness-native" || capture["harness"] != "claude-code" {
		t.Fatalf("capture = %v", capture)
	}
	if _, ok := pred["runtime"]; !ok {
		t.Fatal("harness-native capture must bind runtime trace references")
	}
	// No raw content is ever captured.
	privacy := pred["privacy"].(map[string]any)
	if privacy["rawPrompt"].(map[string]any)["stored"] != false {
		t.Fatal("raw prompt must not be stored")
	}

	res := verifyEvidenceGrade(t, stmt)
	if !res.Valid || res.Level != "evidence-grade" {
		t.Fatalf("expected valid evidence-grade, got %+v", res)
	}
}

func TestCaptureRequiresBoundTrace(t *testing.T) {
	meta := baseMeta()
	meta.TraceIDs = nil
	if _, err := Statement(sampleResult(), meta); err == nil {
		t.Fatal("expected error when harness-native capture has no trace IDs")
	}
	meta = baseMeta()
	meta.TraceDigest = ""
	if _, err := Statement(sampleResult(), meta); err == nil {
		t.Fatal("expected error when trace digest is missing")
	}
}

func TestCaptureRejectsMalformedTraceDigest(t *testing.T) {
	meta := baseMeta()
	meta.TraceDigest = "not-hex"
	if _, err := Statement(sampleResult(), meta); err == nil {
		t.Fatal("expected error for a malformed trace digest")
	}
}

func TestCaptureBindsAgentConfigByDigest(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "AGENTS.md")
	content := []byte("build: go test ./...\nstyle: minimal\n")
	if err := os.WriteFile(cfg, content, 0o644); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(content)

	meta := baseMeta()
	meta.AgentConfigPath = cfg
	stmt, err := Statement(sampleResult(), meta)
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	pred := stmt["predicate"].(map[string]any)
	cfgObj := pred["agentConfig"].(map[string]any)
	if cfgObj["primaryFile"] != "AGENTS.md" {
		t.Fatalf("primaryFile = %v", cfgObj["primaryFile"])
	}
	if got := cfgObj["primaryDigest"].(map[string]any)["sha256"]; got != hex.EncodeToString(want[:]) {
		t.Fatalf("primaryDigest = %v", got)
	}
}

func TestCaptureReferenceIntegrations(t *testing.T) {
	// A local CLI harness (Claude Code style).
	cli := baseMeta()
	cliStmt, err := Statement(sampleResult(), cli)
	if err != nil {
		t.Fatalf("cli harness: %v", err)
	}
	if res := verifyEvidenceGrade(t, cliStmt); !res.Valid {
		t.Fatalf("cli harness statement invalid: %+v", res)
	}

	// A platform coding agent (executionType/invocationKind platform-agent),
	// with an MCP server and a tool bound by schema digest.
	platform := baseMeta()
	platform.AgentName = "copilot-coding-agent"
	platform.InvocationKind = "platform-agent"
	platform.ExecutionType = "platform-agent"
	platform.MCPServers = []MCPServer{{Name: "filesystem", Transport: "stdio", Digest: strings.Repeat("2", 64)}}
	platform.Tools = []Tool{{Name: "read_file", Server: "filesystem", SchemaDigest: strings.Repeat("3", 64)}}
	platformStmt, err := Statement(sampleResult(), platform)
	if err != nil {
		t.Fatalf("platform agent: %v", err)
	}
	pred := platformStmt["predicate"].(map[string]any)
	if pred["environment"].(map[string]any)["executionType"] != "platform-agent" {
		t.Fatalf("executionType = %v", pred["environment"])
	}
	if len(pred["mcpServers"].([]map[string]any)) != 1 || len(pred["tools"].([]map[string]any)) != 1 {
		t.Fatalf("mcp/tools not bound: %+v", pred)
	}
	if res := verifyEvidenceGrade(t, platformStmt); !res.Valid {
		t.Fatalf("platform agent statement invalid: %+v", res)
	}
}
