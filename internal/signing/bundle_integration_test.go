package signing

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/predicate"
	"agentattest.dev/agentattest/internal/verify"
)

// Must match the workload URI SAN minted by newCAChain.
const testWorkflowURI = "https://github.com/example/agentattest/.github/workflows/agentattest.yml@refs/heads/main"

func policyGradeStatement(t *testing.T, declaredIdentity string) []byte {
	t.Helper()
	result := gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: strings.Repeat("a", 64)},
	}
	pred := predicate.CreateV1(result, predicate.CreateOptions{
		AgentName:     "example-agent",
		AgentVersion:  "1.0.0",
		Now:           time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC),
		CaptureMethod: "wrapper",
	})
	pred["verificationLevel"] = "policy-grade"
	agent := pred["agent"].(map[string]any)
	agent["invocationKind"] = "ci-step"
	if declaredIdentity != "" {
		agent["declaredIdentity"] = declaredIdentity
	}
	pred["environment"] = map[string]any{
		"executionType": "github-actions",
		"builder": map[string]any{
			"id":          testWorkflowURI,
			"type":        "github-actions-workflow",
			"workflowRef": testWorkflowURI,
			"runnerClass": "github-hosted",
		},
	}
	evidence := pred["evidence"].([]map[string]any)
	evidence[0]["storage"] = "artifact-store"

	stmt := predicate.StatementForV1(result, pred)
	out, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func bundlePolicyGradeContext(t *testing.T, payload []byte, verified VerifiedContext) []byte {
	t.Helper()
	var stmt map[string]any
	if err := json.Unmarshal(payload, &stmt); err != nil {
		t.Fatal(err)
	}
	subject := stmt["subject"].([]any)[0].(map[string]any)
	sha := subject["digest"].(map[string]any)["sha256"].(string)
	ctxMap := map[string]any{
		"repoUrl":       "https://github.com/example/agentattest",
		"baseCommit":    "0123456789abcdef0123456789abcdef01234567",
		"requiredLevel": "policy-grade",
		"subjects":      []map[string]any{{"name": "patch.diff", "algorithm": "sha256", "digest": sha}},
	}
	for k, v := range verified.ToContextMap() {
		ctxMap[k] = v
	}
	out, err := json.Marshal(ctxMap)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestBundleVerifiedIdentityDrivesPolicy proves that the identity established by
// verifying the bundle's certificate chain — not the predicate's self-asserted
// fields — is what the policy enforces.
func TestBundleVerifiedIdentityDrivesPolicy(t *testing.T) {
	ctx := context.Background()

	// Clean claim: no declaredIdentity. The verified bundle identity satisfies
	// the policy-grade signer/builder/workflow/issuer gates.
	cleanStmt := policyGradeStatement(t, "")
	cleanBundle, cleanRoot, _ := makeBundle(t, cleanStmt)
	cleanVerified, cleanPayload, err := FromSigstoreBundle(ctx, cleanBundle, TrustRoot{Roots: []*x509.Certificate{cleanRoot}})
	if err != nil {
		t.Fatalf("FromSigstoreBundle (clean): %v", err)
	}
	if cleanVerified.VerifiedWorkflowRef != testWorkflowURI {
		t.Fatalf("verified workflowRef = %q, want %q", cleanVerified.VerifiedWorkflowRef, testWorkflowURI)
	}
	res := verify.Predicate(ctx, cleanPayload, bundlePolicyGradeContext(t, cleanPayload, cleanVerified), verify.Options{})
	if !res.Valid || res.Level != "policy-grade" {
		t.Fatalf("expected valid policy-grade from verified bundle, got %+v", res)
	}

	// Lying claim: the predicate declares a different signer than the bundle
	// certificate verifies to. The verified identity wins → signer_identity_mismatch.
	lieStmt := policyGradeStatement(t, "rogue-identity")
	lieBundle, lieRoot, _ := makeBundle(t, lieStmt)
	lieVerified, liePayload, err := FromSigstoreBundle(ctx, lieBundle, TrustRoot{Roots: []*x509.Certificate{lieRoot}})
	if err != nil {
		t.Fatalf("FromSigstoreBundle (lie): %v", err)
	}
	lieRes := verify.Predicate(ctx, liePayload, bundlePolicyGradeContext(t, liePayload, lieVerified), verify.Options{})
	if lieRes.Valid {
		t.Fatal("expected the self-asserted signer to be rejected against the verified bundle identity")
	}
	if len(lieRes.FailureCodes) != 1 || lieRes.FailureCodes[0] != "signer_identity_mismatch" {
		t.Fatalf("failureCodes = %v, want [signer_identity_mismatch]", lieRes.FailureCodes)
	}
}
