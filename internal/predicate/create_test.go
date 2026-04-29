package predicate

import (
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
