package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"
)

// selfSignedPEM builds a throwaway self-signed certificate PEM for tests.
func selfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-root"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

type fakeTUF struct {
	targets   map[string][]byte
	refreshed bool
}

func (f *fakeTUF) Refresh() error { f.refreshed = true; return nil }

func (f *fakeTUF) GetTarget(name string) ([]byte, error) {
	blob, ok := f.targets[name]
	if !ok {
		return nil, errMissingTarget
	}
	return blob, nil
}

var errMissingTarget = &missingTargetError{}

type missingTargetError struct{}

func (*missingTargetError) Error() string { return "target not found" }

func TestLoadTrustRootExplicitPEMWins(t *testing.T) {
	pemBytes := selfSignedPEM(t)
	roots, err := LoadTrustRoot(context.Background(), TrustRootOptions{
		ExplicitPEM: pemBytes,
		CacheDir:    t.TempDir(), // must not be touched when explicit PEM wins
	})
	if err != nil {
		t.Fatalf("explicit trust root rejected: %v", err)
	}
	if len(roots) != 1 || roots[0].Subject.CommonName != "test-root" {
		t.Fatalf("unexpected roots: %+v", roots)
	}
}

func TestRootsFromFetcherPrefersV1Target(t *testing.T) {
	pemBytes := selfSignedPEM(t)
	roots, err := rootsFromFetcher(&fakeTUF{targets: map[string][]byte{
		"fulcio_v1.crt.pem": pemBytes,
		"fulcio.crt.pem":    []byte("garbage"),
	}})
	if err != nil {
		t.Fatalf("v1 target rejected: %v", err)
	}
	if len(roots) != 1 {
		t.Fatalf("want 1 root, got %d", len(roots))
	}
}

func TestRootsFromFetcherFallsBackToLegacyTarget(t *testing.T) {
	pemBytes := selfSignedPEM(t)
	roots, err := rootsFromFetcher(&fakeTUF{targets: map[string][]byte{
		"fulcio.crt.pem": pemBytes,
	}})
	if err != nil {
		t.Fatalf("legacy target rejected: %v", err)
	}
	if len(roots) != 1 {
		t.Fatalf("want 1 root, got %d", len(roots))
	}
}

func TestRootsFromFetcherFailsClosedOnMissingTargets(t *testing.T) {
	if _, err := rootsFromFetcher(&fakeTUF{targets: map[string][]byte{}}); err == nil {
		t.Fatal("expected failure when no Fulcio target exists")
	}
}

func TestRootsFromFetcherFailsClosedOnEmptyCertList(t *testing.T) {
	// Valid PEM blocks but no CERTIFICATE entries.
	notACert := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("x")})
	if _, err := rootsFromFetcher(&fakeTUF{targets: map[string][]byte{"fulcio_v1.crt.pem": notACert}}); err == nil {
		t.Fatal("expected failure when target parses to zero certificates")
	}
}

func TestDefaultTUFCacheDirUnderUserCache(t *testing.T) {
	dir := DefaultTUFCacheDir()
	if dir == "" {
		t.Fatal("empty default TUF cache dir")
	}
	if base, err := os.UserCacheDir(); err == nil && dir == base {
		t.Fatalf("default TUF cache dir should be namespaced under the user cache dir, got %q", dir)
	}
}

// TestFulcioRootsViaTUFIntegration exercises the real Sigstore TUF repository.
// It is opt-in because it requires network access:
//
//	AGENTATTEST_TUF_INTEGRATION=1 go test ./internal/signing/ -run Integration
func TestFulcioRootsViaTUFIntegration(t *testing.T) {
	if os.Getenv("AGENTATTEST_TUF_INTEGRATION") == "" {
		t.Skip("set AGENTATTEST_TUF_INTEGRATION=1 to run against the live Sigstore TUF repository")
	}
	roots, err := fulcioRootsViaTUF(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("live TUF fetch failed: %v", err)
	}
	if len(roots) == 0 {
		t.Fatal("live TUF fetch returned no roots")
	}
	for _, root := range roots {
		t.Logf("fulcio root: CN=%q expires=%s", root.Subject.CommonName, root.NotAfter.Format(time.RFC3339))
	}
}
