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
}

func Create(result gitbind.Result, opts CreateOptions) map[string]any {
	now := opts.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	repoURL := firstNonEmpty(opts.RepoURL, result.RepoURL)
	baseCommit := firstNonEmpty(opts.BaseCommit, result.BaseCommit)
	runID := fmt.Sprintf("run_%s_%d", now.Format("20060102T150405Z"), os.Getpid())
	agentName := firstNonEmpty(opts.AgentName, "agentattest-cli")
	agentVersion := firstNonEmpty(opts.AgentVersion, "v0-alpha")

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
		"predicateVersion":  "v0",
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

func StatementFor(result gitbind.Result, predicate map[string]any) map[string]any {
	return statement.New([]statement.Subject{
		{
			Name: "patch.diff",
			Digest: map[string]string{
				result.Patch.Algorithm: result.Patch.Digest,
			},
		},
	}, predicate)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
