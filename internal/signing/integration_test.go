package signing

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"testing"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"
	"agentattest.dev/agentattest/internal/predicate"
	"agentattest.dev/agentattest/internal/verify"
)

// TestSignedPolicyGradeV1Verifies is the end-to-end proof of task 7: a v1
// statement is DSSE-signed by a workload-identity certificate, the adapter
// verifies the envelope into VerifiedContext, that context is merged with the
// out-of-band repo/subject context, and the full verifier accepts the claim at
// policy-grade.
func TestSignedPolicyGradeV1Verifies(t *testing.T) {
	ctx := context.Background()
	workflow := "https://github.com/example/agentattest/.github/workflows/agentattest.yml@refs/heads/main"
	issuer := "https://token.actions.githubusercontent.com"
	key, cert := newCert(t, mustURLs(t, workflow), nil, issuer)

	result := gitbind.Result{
		RepoURL:    "https://github.com/example/agentattest",
		BaseCommit: "0123456789abcdef0123456789abcdef01234567",
		Patch:      gitbind.Digest{Algorithm: "sha256", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	pred := predicate.CreateV1(result, predicate.CreateOptions{
		AgentName:     "example-agent",
		AgentVersion:  "1.0.0",
		Now:           time.Date(2026, 4, 27, 9, 12, 31, 0, time.UTC),
		CaptureMethod: "wrapper",
	})

	// Elevate the CLI-produced evidence-grade base into a policy-grade
	// GitHub-Actions claim whose declared identity and builder match the
	// signing certificate.
	pred["verificationLevel"] = "policy-grade"
	agent := pred["agent"].(map[string]any)
	agent["invocationKind"] = "ci-step"
	// declaredIdentity is intentionally left unset: a workload-identity signer
	// (the GitHub workflow URI, which contains '@') cannot be expressed in the
	// declaredIdentity pattern, and the verifier only requires verifiedSigner to
	// be present — which the signing certificate supplies below.
	pred["environment"] = map[string]any{
		"executionType": "github-actions",
		"builder": map[string]any{
			"id":          workflow,
			"type":        "github-actions-workflow",
			"workflowRef": workflow,
			"runnerClass": "github-hosted",
		},
	}
	evidence := pred["evidence"].([]map[string]any)
	evidence[0]["storage"] = "artifact-store" // not local-only at policy-grade

	stmt := predicate.StatementForV1(result, pred)
	stmtJSON, err := json.Marshal(stmt)
	if err != nil {
		t.Fatal(err)
	}

	env, err := Sign(ctx, PayloadType, stmtJSON, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	vc, err := Verify(ctx, env, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("verify envelope: %v", err)
	}

	// Integrity: the signed payload is exactly the statement we will verify.
	decoded, err := env.DecodeB64Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, stmtJSON) {
		t.Fatal("decoded DSSE payload does not match the signed statement")
	}

	contextInput := map[string]any{
		"repoUrl":       result.RepoURL,
		"baseCommit":    result.BaseCommit,
		"requiredLevel": "policy-grade",
		"subjects": []map[string]any{
			{"name": "patch.diff", "algorithm": "sha256", "digest": result.Patch.Digest},
		},
	}
	for k, v := range vc.ToContextMap() {
		contextInput[k] = v
	}
	contextJSON, err := json.Marshal(contextInput)
	if err != nil {
		t.Fatal(err)
	}

	got := verify.Predicate(ctx, stmtJSON, contextJSON, verify.Options{})
	if !got.Valid {
		t.Fatalf("expected valid policy-grade, got %+v", got)
	}
	if got.Level != "policy-grade" {
		t.Fatalf("level = %q, want policy-grade", got.Level)
	}
}
