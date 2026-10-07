package agentattest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}

// TestGoldenFixturesThroughWrapper proves the wrapper is behavior-identical to
// the internal verifier on every upstream golden fixture.
func TestGoldenFixturesThroughWrapper(t *testing.T) {
	root := filepath.Join(repoRoot(t), "tests", "golden")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			st, _ := os.ReadFile(filepath.Join(dir, "statement.json"))
			cx, _ := os.ReadFile(filepath.Join(dir, "context.json"))
			var want Result
			b, _ := os.ReadFile(filepath.Join(dir, "want.json"))
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			got := Verify(context.Background(), st, cx)
			if got.Valid != want.Valid || got.Level != want.Level || !reflect.DeepEqual(got.FailureCodes, want.FailureCodes) {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
	if n < 40 {
		t.Fatalf("expected the upstream golden set, found %d fixtures", n)
	}
}

// TestContractDigestsMatchRepository detects drift between the embedded pack
// and the files in the repository.
func TestContractDigestsMatchRepository(t *testing.T) {
	d := ContractDigests()
	if len(d) != 6 {
		t.Fatalf("expected 6 contract files, got %d", len(d))
	}
	for p, sum := range d {
		b, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != sum {
			t.Fatalf("%s: embedded bytes differ from the repository file", p)
		}
	}
}

func TestFailureCodesOrdered(t *testing.T) {
	shuffled := append([]string{}, FailureCodes...)
	for i, j := 0, len(shuffled)-1; i < j; i, j = i+1, j-1 {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	if !reflect.DeepEqual(OrderFailureCodes(shuffled), FailureCodes) {
		t.Fatal("FailureCodes is not in upstream priority order")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestBindRangeRoundTrip creates a statement for a committed change and
// verifies it at evidence-grade, then proves a tampered subject fails.
func TestBindRangeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "dev@example.invalid")
	git(t, dir, "config", "user.name", "Dev")
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "base")
	base := git(t, dir, "rev-parse", "HEAD")
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("two\n"), 0o644)
	git(t, dir, "commit", "-q", "-am", "change")
	head := git(t, dir, "rev-parse", "HEAD")

	b, err := BindRange(context.Background(), dir, "https://github.com/example/app", base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.ChangedFiles) != 1 || b.ChangedFiles[0].Path != "a.txt" || b.PatchSHA256 == "" {
		t.Fatalf("unexpected binding %+v", b)
	}
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	st, err := NewStatementV1(b, StatementOptions{AgentName: "example-agent", AgentVersion: "1.0.0", CaptureMethod: "ci-step", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	cx, err := VerifierContext(b, ContextOptions{RequiredLevel: "evidence-grade", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if r := Verify(context.Background(), st, cx); !r.Valid {
		t.Fatalf("round trip failed: %+v", r)
	}

	tampered := b
	tampered.PatchSHA256 = strings.Repeat("0", 64)
	cx2, _ := VerifierContext(tampered, ContextOptions{RequiredLevel: "evidence-grade", Now: now})
	if r := Verify(context.Background(), st, cx2); r.Valid || !reflect.DeepEqual(r.FailureCodes, []string{"subject_digest_mismatch"}) {
		t.Fatalf("tampered subject must fail with subject_digest_mismatch: %+v", r)
	}
}

func TestVerifierContextRejectsUnverifiedKeys(t *testing.T) {
	_, err := VerifierContext(GitBinding{RepoURL: "https://github.com/a/b", BaseCommit: "x", PatchSHA256: "y"},
		ContextOptions{Verified: map[string]any{"repoUrl": "https://evil.example"}})
	if err == nil {
		t.Fatal("non-verified keys must be rejected")
	}
}

// TestPolicyGradeCIStatement proves a CI-shaped statement reaches policy-grade
// only with matching verified identity, and fails closed otherwise.
func TestPolicyGradeCIStatement(t *testing.T) {
	b := GitBinding{RepoURL: "https://github.com/example/app", BaseCommit: strings.Repeat("a", 40), HeadCommit: strings.Repeat("b", 40), PatchSHA256: strings.Repeat("c", 64)}
	wf := "https://github.com/example/app/.github/workflows/provenance.yml@refs/heads/main"
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	st, err := NewStatementV1(b, StatementOptions{
		AgentName: "example-agent", AgentVersion: "1.0.0", CaptureMethod: "ci-step", InvocationKind: "ci-step",
		ExecutionType: "github-actions", VerificationLevel: "policy-grade", EvidenceStorage: "artifact-store",
		Builder: &Builder{ID: wf, Type: "github-actions-workflow", WorkflowRef: wf, RunnerClass: "github-hosted"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	verified := map[string]any{"verifiedSigner": wf, "verifiedBuilderId": wf, "verifiedWorkflowRef": wf, "verifiedIssuer": "https://token.actions.githubusercontent.com"}
	cx, _ := VerifierContext(b, ContextOptions{RequiredLevel: "policy-grade", Verified: verified, Now: now})
	if r := Verify(context.Background(), st, cx); !r.Valid || r.Level != "policy-grade" {
		t.Fatalf("policy-grade CI statement must verify: %+v", r)
	}
	other := map[string]any{"verifiedSigner": wf, "verifiedBuilderId": "https://github.com/evil/x/.github/workflows/y.yml@refs/heads/main",
		"verifiedWorkflowRef": "https://github.com/evil/x/.github/workflows/y.yml@refs/heads/main", "verifiedIssuer": "https://token.actions.githubusercontent.com"}
	cx2, _ := VerifierContext(b, ContextOptions{RequiredLevel: "policy-grade", Verified: other, Now: now})
	if r := Verify(context.Background(), st, cx2); r.Valid {
		t.Fatalf("mismatched workflow identity must fail: %+v", r)
	}
	cx3, _ := VerifierContext(b, ContextOptions{RequiredLevel: "policy-grade", Now: now})
	if r := Verify(context.Background(), st, cx3); r.Valid {
		t.Fatalf("policy-grade without verified identity must fail: %+v", r)
	}
}
