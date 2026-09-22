package signing

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"
)

// makeBundle signs statement with a fresh workload-identity leaf and wraps the
// DSSE envelope + certificate chain into a Sigstore-bundle-shaped JSON, matching
// what cosign / actions/attest emit.
func makeBundle(t *testing.T, statement []byte) (bundleJSON []byte, root *x509.Certificate, issuer string) {
	t.Helper()
	issuer = "https://token.actions.githubusercontent.com"
	leafKey, leaf, inter, root := newCAChain(t, issuer, false)

	env, err := Sign(context.Background(), PayloadType, statement, leafKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	envJSON, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var envObj map[string]any
	if err := json.Unmarshal(envJSON, &envObj); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	bundle := map[string]any{
		"mediaType": "application/vnd.dev.sigstore.bundle+json;version=0.3",
		"verificationMaterial": map[string]any{
			"x509CertificateChain": map[string]any{
				"certificates": []string{
					base64.StdEncoding.EncodeToString(leaf.Raw),
					base64.StdEncoding.EncodeToString(inter.Raw),
				},
			},
		},
		"dsseEnvelope": envObj,
	}
	out, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	return out, root, issuer
}

func TestFromSigstoreBundle(t *testing.T) {
	ctx := context.Background()
	statement := []byte(`{"_type":"https://in-toto.io/Statement/v1","predicateType":"https://agentattest.dev/predicate/v1"}`)
	bundleJSON, root, issuer := makeBundle(t, statement)

	vc, payload, err := FromSigstoreBundle(ctx, bundleJSON, TrustRoot{Roots: []*x509.Certificate{root}, AllowedIssuers: []string{issuer}})
	if err != nil {
		t.Fatalf("FromSigstoreBundle: %v", err)
	}
	if string(payload) != string(statement) {
		t.Fatalf("decoded payload = %q, want %q", payload, statement)
	}
	if vc.VerifiedIssuer != issuer {
		t.Fatalf("verifiedIssuer = %q, want %q", vc.VerifiedIssuer, issuer)
	}
	if vc.VerifiedWorkflowRef == "" || vc.VerifiedBuilderID == "" || vc.VerifiedSigner == "" {
		t.Fatalf("expected workload identity fields, got %+v", vc)
	}
}

func TestFromSigstoreBundleGitHubWrapper(t *testing.T) {
	ctx := context.Background()
	statement := []byte(`{"_type":"https://in-toto.io/Statement/v1"}`)
	bundleJSON, root, issuer := makeBundle(t, statement)

	// Wrap in the `gh attestation verify --format json` attestations[] shape.
	var inner map[string]any
	if err := json.Unmarshal(bundleJSON, &inner); err != nil {
		t.Fatal(err)
	}
	wrapped, err := json.Marshal([]map[string]any{
		{"type": "https://agentattest.dev/predicate/v1", "attestations": []map[string]any{{"bundle": inner}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	vc, payload, err := FromSigstoreBundle(ctx, wrapped, TrustRoot{Roots: []*x509.Certificate{root}})
	if err != nil {
		t.Fatalf("FromSigstoreBundle (gh wrapper): %v", err)
	}
	if string(payload) != string(statement) {
		t.Fatalf("decoded payload = %q", payload)
	}
	if vc.VerifiedIssuer != issuer {
		t.Fatalf("verifiedIssuer = %q", vc.VerifiedIssuer)
	}
}

func TestFromSigstoreBundleRejectsUntrustedRoot(t *testing.T) {
	ctx := context.Background()
	bundleJSON, _, _ := makeBundle(t, []byte(`{"_type":"https://in-toto.io/Statement/v1"}`))
	_, _, _, otherRoot := newCAChain(t, "https://issuer.example", false)
	if _, _, err := FromSigstoreBundle(ctx, bundleJSON, TrustRoot{Roots: []*x509.Certificate{otherRoot}}); err == nil {
		t.Fatal("expected failure against an untrusted root")
	}
}

func TestFromSigstoreBundleNoEnvelope(t *testing.T) {
	ctx := context.Background()
	_, root, _ := makeBundle(t, []byte("x"))
	bundle := []byte(`{"verificationMaterial":{"x509CertificateChain":{"certificates":[]}}}`)
	if _, _, err := FromSigstoreBundle(ctx, bundle, TrustRoot{Roots: []*x509.Certificate{root}}); err == nil {
		t.Fatal("expected error when no dsseEnvelope is present")
	}
}
