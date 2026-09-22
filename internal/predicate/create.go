package predicate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/statement"
)

type CreateOptions struct {
	AgentName    string
	AgentVersion string
	RepoURL      string
	BaseCommit   string
	Now          time.Time

	// v1-only options. They are ignored by Create (which always emits v0).
	CaptureMethod     string // wrapper | ci-step | manual (harness-native is produced by capture integrations, not the CLI)
	CaptureHarness    string // harness name; schema/CUE require it when CaptureMethod == "harness-native"
	AgentConfigPath   string // display name of the bound operating contract (e.g. AGENTS.md)
	AgentConfigSHA256 string // hex sha256 of the operating-contract file
	ModelProvider     string
	ModelID           string
}

// Create builds a v0 predicate (https://agentattest.dev/predicate/v0).
func Create(result gitbind.Result, opts CreateOptions) map[string]any {
	pred := basePredicate(result, opts, "v0-alpha")
	pred["predicateVersion"] = "v0"
	return pred
}

// CreateV1 builds a v1 predicate (https://agentattest.dev/predicate/v1): the v0
// base plus a capture descriptor, an optional bound operating contract
// (agentConfig), and an optional self-asserted model block. It never stores raw
// prompts or raw tool outputs.
func CreateV1(result gitbind.Result, opts CreateOptions) map[string]any {
	pred := basePredicate(result, opts, "v1-alpha")
	pred["predicateVersion"] = "v1"
	pred["capture"] = captureObject(opts)
	if cfg := agentConfigObject(opts); cfg != nil {
		pred["agentConfig"] = cfg
	}
	if m := modelObject(opts); m != nil {
		pred["model"] = m
	}
	return pred
}

func basePredicate(result gitbind.Result, opts CreateOptions, defaultAgentVersion string) map[string]any {
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	repoURL := firstNonEmpty(opts.RepoURL, result.RepoURL)
	baseCommit := firstNonEmpty(opts.BaseCommit, result.BaseCommit)
	runID := fmt.Sprintf("run_%s_%d", now.Format("20060102T150405Z"), os.Getpid())
	agentName := firstNonEmpty(opts.AgentName, "agentattest-cli")
	agentVersion := firstNonEmpty(opts.AgentVersion, defaultAgentVersion)

	evidenceDigestInput := []byte(runID + "\n" + result.Patch.Digest + "\n")
	evidenceSum := sha256.Sum256(evidenceDigestInput)

	repo := map[string]any{
		"url":        repoURL,
		"baseCommit": baseCommit,
	}
	if result.HeadCommit != "" {
		repo["headCommit"] = result.HeadCommit
	}
	if result.Branch != "" {
		repo["branch"] = result.Branch
	}

	return map[string]any{
		"runId":             runID,
		"verificationLevel": "evidence-grade",
		"repo":              repo,
		"agent": map[string]any{
			"name":           agentName,
			"version":        agentVersion,
			"invocationKind": "local-cli",
		},
		"environment": map[string]any{
			"executionType": "local",
		},
		"humanApproval": map[string]any{
			"state": "none",
		},
		"privacy": map[string]any{
			"redactionPolicy": "default-minimal-v0",
			"rawPrompt": map[string]any{
				"stored":     false,
				"visibility": "none",
			},
			"rawToolOutputs": map[string]any{
				"stored":     false,
				"visibility": "none",
			},
			"publicTransparencyLog": map[string]any{
				"allowed":            false,
				"includesRawContent": false,
			},
		},
		"timestamps": map[string]any{
			"startedAt":  now.Format(time.RFC3339),
			"finishedAt": now.Format(time.RFC3339),
		},
		"evidence": []map[string]any{
			{
				"type":       "tool-summary",
				"uri":        "artifact://agentattest/evidence/" + runID + ".summary.json",
				"digest":     map[string]any{"sha256": hex.EncodeToString(evidenceSum[:])},
				"mediaType":  "application/json",
				"storage":    "local-only",
				"visibility": "team",
			},
		},
	}
}

func captureObject(opts CreateOptions) map[string]any {
	method := opts.CaptureMethod
	if method == "" {
		method = "wrapper"
	}
	capture := map[string]any{"method": method}
	if opts.CaptureHarness != "" {
		capture["harness"] = opts.CaptureHarness
	}
	return capture
}

func agentConfigObject(opts CreateOptions) map[string]any {
	if opts.AgentConfigSHA256 == "" {
		return nil
	}
	primaryFile := opts.AgentConfigPath
	if primaryFile == "" {
		primaryFile = "AGENTS.md"
	}
	return map[string]any{
		"primaryFile":   primaryFile,
		"primaryDigest": map[string]any{"sha256": opts.AgentConfigSHA256},
	}
}

func modelObject(opts CreateOptions) map[string]any {
	if opts.ModelProvider == "" || opts.ModelID == "" {
		return nil
	}
	return map[string]any{
		"provider": opts.ModelProvider,
		"modelId":  opts.ModelID,
	}
}

// StatementFor wraps a v0 predicate in an in-toto Statement v1.
func StatementFor(result gitbind.Result, predicateMap map[string]any) map[string]any {
	return statement.New(patchSubject(result), predicateMap)
}

// StatementForV1 wraps a v1 predicate in an in-toto Statement v1 whose
// predicateType is https://agentattest.dev/predicate/v1.
func StatementForV1(result gitbind.Result, predicateMap map[string]any) map[string]any {
	return statement.NewV1(patchSubject(result), predicateMap)
}

func patchSubject(result gitbind.Result) []statement.Subject {
	return []statement.Subject{
		{
			Name: "patch.diff",
			Digest: map[string]string{
				result.Patch.Algorithm: result.Patch.Digest,
			},
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
