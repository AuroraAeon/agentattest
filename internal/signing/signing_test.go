package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const testFulcioIssuerOID = "1.3.6.1.4.1.57264.1.1"

// newCert mints a self-signed certificate with the given URI/email SANs and an
// optional Fulcio OIDC issuer extension, returning the matching private key.
func newCert(t *testing.T, uris []*url.URL, emails []string, issuer string) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var extra []pkix.Extension
	if issuer != "" {
		extra = append(extra, pkix.Extension{
			Id:    asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1},
			Value: []byte(issuer),
		})
	}
	tmpl := &x509.Certificate{
		SerialNumber:    big.NewInt(1),
		Subject:         pkix.Name{CommonName: "test-signer"},
		NotBefore:       time.Now().Add(-time.Hour),
		NotAfter:        time.Now().Add(time.Hour),
		KeyUsage:        x509.KeyUsageDigitalSignature,
		ExtKeyUsage:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:            uris,
		EmailAddresses:  emails,
		ExtraExtensions: extra,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return key, cert
}

func mustURLs(t *testing.T, raws ...string) []*url.URL {
	t.Helper()
	out := make([]*url.URL, 0, len(raws))
	for _, raw := range raws {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse url %q: %v", raw, err)
		}
		out = append(out, u)
	}
	return out
}

func TestSignVerifyRoundTripWorkloadIdentity(t *testing.T) {
	ctx := context.Background()
	workflow := "https://github.com/example/agentattest/.github/workflows/agentattest.yml@refs/heads/main"
	issuer := "https://token.actions.githubusercontent.com"
	key, cert := newCert(t, mustURLs(t, workflow), nil, issuer)

	env, err := Sign(ctx, PayloadType, []byte(`{"_type":"https://in-toto.io/Statement/v1"}`), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	vc, err := Verify(ctx, env, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if vc.VerifiedSigner != workflow {
		t.Fatalf("verifiedSigner = %q, want %q", vc.VerifiedSigner, workflow)
	}
	if vc.VerifiedWorkflowRef != workflow || vc.VerifiedBuilderID != workflow {
		t.Fatalf("workload identity must fill builder/workflow, got %+v", vc)
	}
	if vc.VerifiedIssuer != issuer {
		t.Fatalf("verifiedIssuer = %q, want %q", vc.VerifiedIssuer, issuer)
	}
}

func TestSignVerifyEmailIdentity(t *testing.T) {
	ctx := context.Background()
	key, cert := newCert(t, nil, []string{"dev@example.com"}, "")

	env, err := Sign(ctx, "", []byte("hello"), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	vc, err := Verify(ctx, env, []*x509.Certificate{cert})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if vc.VerifiedSigner != "dev@example.com" {
		t.Fatalf("verifiedSigner = %q", vc.VerifiedSigner)
	}
	if vc.VerifiedBuilderID != "" || vc.VerifiedWorkflowRef != "" {
		t.Fatalf("email identity must not set builder/workflow: %+v", vc)
	}
	if vc.VerifiedIssuer != "" {
		t.Fatalf("verifiedIssuer should be empty, got %q", vc.VerifiedIssuer)
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	ctx := context.Background()
	key, cert := newCert(t, mustURLs(t, "https://github.com/example/agentattest/.github/workflows/w.yml@refs/heads/main"), nil, "")
	env, err := Sign(ctx, PayloadType, []byte("original"), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	env.Payload = base64.StdEncoding.EncodeToString([]byte("tampered"))
	if _, err := Verify(ctx, env, []*x509.Certificate{cert}); err == nil {
		t.Fatal("expected verification to fail on tampered payload")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	ctx := context.Background()
	signKey, _ := newCert(t, nil, []string{"signer@example.com"}, "")
	_, otherCert := newCert(t, nil, []string{"other@example.com"}, "")
	env, err := Sign(ctx, PayloadType, []byte("payload"), signKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := Verify(ctx, env, []*x509.Certificate{otherCert}); err == nil {
		t.Fatal("expected verification to fail when trusted cert does not match the signer")
	}
}

func TestVerifyRejectsNoTrustedCerts(t *testing.T) {
	ctx := context.Background()
	key, _ := newCert(t, nil, []string{"a@example.com"}, "")
	env, err := Sign(ctx, PayloadType, []byte("payload"), key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := Verify(ctx, env, nil); err == nil {
		t.Fatal("expected error with no trusted certificates")
	}
}

func TestVerifyRejectsMissingSignature(t *testing.T) {
	ctx := context.Background()
	_, cert := newCert(t, nil, []string{"a@example.com"}, "")
	env := &dsse.Envelope{
		PayloadType: PayloadType,
		Payload:     base64.StdEncoding.EncodeToString([]byte("x")),
	}
	if _, err := Verify(ctx, env, []*x509.Certificate{cert}); err == nil {
		t.Fatal("expected error with no signatures")
	}
}
