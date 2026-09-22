package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// newCAChain mints a root CA -> intermediate -> leaf chain. The leaf carries a
// GitHub Actions workload-identity URI SAN and a Fulcio OIDC issuer extension.
// When leafExpired is true the leaf is already outside its validity window.
func newCAChain(t *testing.T, issuer string, leafExpired bool) (leafKey *ecdsa.PrivateKey, leaf, intermediate, root *x509.Certificate) {
	t.Helper()

	mkCA := func(cn string, serial int64, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*ecdsa.PrivateKey, *x509.Certificate) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("generate key: %v", err)
		}
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(serial),
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(24 * time.Hour),
			IsCA:                  true,
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
		}
		if parent == nil { // self-signed root
			parent = tmpl
			parentKey = key
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
		if err != nil {
			t.Fatalf("create %s: %v", cn, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatalf("parse %s: %v", cn, err)
		}
		return key, cert
	}

	rootKey, root := mkCA("test-root-ca", 1, nil, nil)
	interKey, intermediate := mkCA("test-intermediate", 2, root, rootKey)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafNotAfter := time.Now().Add(time.Hour)
	if leafExpired {
		leafNotAfter = time.Now().Add(-time.Minute)
	}
	workflow := "https://github.com/example/agentattest/.github/workflows/agentattest.yml@refs/heads/main"
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "leaf"},
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     leafNotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		URIs:         mustURLs(t, workflow),
		ExtraExtensions: []pkix.Extension{
			{Id: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}, Value: []byte(issuer)},
		},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, intermediate, &leafKey.PublicKey, interKey)
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}
	leaf, err = x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	return leafKey, leaf, intermediate, root
}

func TestVerifyWithTrustRootAcceptsValidChain(t *testing.T) {
	ctx := context.Background()
	issuer := "https://token.actions.githubusercontent.com"
	leafKey, leaf, inter, root := newCAChain(t, issuer, false)

	env, err := Sign(ctx, PayloadType, []byte("payload"), leafKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	vc, err := VerifyWithTrustRoot(ctx, env, leaf, []*x509.Certificate{inter}, TrustRoot{
		Roots:          []*x509.Certificate{root},
		AllowedIssuers: []string{issuer},
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if vc.VerifiedIssuer != issuer {
		t.Fatalf("verifiedIssuer = %q, want %q", vc.VerifiedIssuer, issuer)
	}
	if vc.VerifiedSigner == "" || vc.VerifiedWorkflowRef == "" || vc.VerifiedBuilderID == "" {
		t.Fatalf("expected workload identity fields, got %+v", vc)
	}
}

func TestVerifyWithTrustRootRejectsUntrustedRoot(t *testing.T) {
	ctx := context.Background()
	leafKey, leaf, inter, _ := newCAChain(t, "https://issuer.example", false)
	_, _, _, otherRoot := newCAChain(t, "https://issuer.example", false)

	env, err := Sign(ctx, PayloadType, []byte("payload"), leafKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := VerifyWithTrustRoot(ctx, env, leaf, []*x509.Certificate{inter}, TrustRoot{Roots: []*x509.Certificate{otherRoot}}); err == nil {
		t.Fatal("expected chain verification to fail against an untrusted root")
	}
}

func TestVerifyWithTrustRootRejectsDisallowedIssuer(t *testing.T) {
	ctx := context.Background()
	leafKey, leaf, inter, root := newCAChain(t, "https://evil.example", false)

	env, err := Sign(ctx, PayloadType, []byte("payload"), leafKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	_, err = VerifyWithTrustRoot(ctx, env, leaf, []*x509.Certificate{inter}, TrustRoot{
		Roots:          []*x509.Certificate{root},
		AllowedIssuers: []string{"https://token.actions.githubusercontent.com"},
	})
	if err == nil {
		t.Fatal("expected issuer allowlist to reject a disallowed issuer")
	}
}

func TestVerifyWithTrustRootRejectsExpiredLeaf(t *testing.T) {
	ctx := context.Background()
	issuer := "https://token.actions.githubusercontent.com"
	leafKey, leaf, inter, root := newCAChain(t, issuer, true)

	env, err := Sign(ctx, PayloadType, []byte("payload"), leafKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := VerifyWithTrustRoot(ctx, env, leaf, []*x509.Certificate{inter}, TrustRoot{Roots: []*x509.Certificate{root}}); err == nil {
		t.Fatal("expected chain verification to fail for an expired leaf")
	}
}

func TestParseCertificates(t *testing.T) {
	_, _, _, root := newCAChain(t, "https://issuer.example", false)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw})

	certs, err := ParseCertificates(pemBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(certs) != 1 || !certs[0].Equal(root) {
		t.Fatalf("parsed %d certs, want the original root", len(certs))
	}
	if _, err := ParseCertificates([]byte("-----BEGIN CERTIFICATE-----\nnope\n-----END CERTIFICATE-----\n")); err == nil {
		t.Fatal("expected error on malformed certificate PEM")
	}
	if _, err := ParseCertificates([]byte("not pem at all")); err == nil {
		t.Fatal("expected error on non-PEM input")
	}
}
