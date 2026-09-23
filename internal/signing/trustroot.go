package signing

import (
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sigstore/sigstore-go/pkg/tuf"
)

// fulcioTargets lists the Fulcio root-certificate targets published in the
// Sigstore TUF repository, in preference order.
var fulcioTargets = []string{"fulcio_v1.crt.pem", "fulcio.crt.pem"}

// TrustRootOptions configures LoadTrustRoot.
type TrustRootOptions struct {
	// ExplicitPEM, when non-empty, is the PEM-encoded trusted root CA
	// certificate(s) supplied out of band (e.g. --trust-root). It always
	// wins over the TUF-sourced root.
	ExplicitPEM []byte

	// CacheDir is the Sigstore TUF cache location. Empty selects
	// DefaultTUFCacheDir().
	CacheDir string
}

// LoadTrustRoot resolves the trusted root CA set for bundle verification.
// With ExplicitPEM it parses that PEM; otherwise it fetches the Sigstore
// community Fulcio root through the Sigstore TUF repository, whose root
// metadata is pinned by the copy embedded in sigstore-go. Every failure
// (network, repository, malformed target) fails closed.
func LoadTrustRoot(ctx context.Context, opts TrustRootOptions) ([]*x509.Certificate, error) {
	if len(opts.ExplicitPEM) > 0 {
		return ParseCertificates(opts.ExplicitPEM)
	}
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		cacheDir = DefaultTUFCacheDir()
	}
	return fulcioRootsViaTUF(ctx, cacheDir)
}

// DefaultTUFCacheDir is where the Sigstore TUF metadata cache lives:
// os.UserCacheDir()/agentattest/tuf (honors XDG_CACHE_HOME on Linux).
func DefaultTUFCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "agentattest", "tuf")
}

func fulcioRootsViaTUF(ctx context.Context, cacheDir string) ([]*x509.Certificate, error) {
	client, err := tuf.New(tuf.DefaultOptions().WithContext(ctx).WithCachePath(cacheDir))
	if err != nil {
		return nil, fmt.Errorf("signing: create Sigstore TUF client: %w", err)
	}
	if err := client.Refresh(); err != nil {
		return nil, fmt.Errorf("signing: refresh Sigstore TUF repository (network or repository unavailable): %w", err)
	}
	return rootsFromFetcher(client)
}

// tufTargets is the seam over the Sigstore TUF client so tests stay hermetic.
type tufTargets interface {
	Refresh() error
	GetTarget(name string) ([]byte, error)
}

// rootsFromFetcher extracts the Fulcio root certificate chain from the TUF
// targets, trying each candidate name in order.
func rootsFromFetcher(f tufTargets) ([]*x509.Certificate, error) {
	var lastErr error
	for _, name := range fulcioTargets {
		blob, err := f.GetTarget(name)
		if err != nil {
			lastErr = err
			continue
		}
		roots, err := ParseCertificates(blob)
		if err != nil {
			lastErr = err
			continue
		}
		if len(roots) == 0 {
			lastErr = fmt.Errorf("target %q contains no certificates", name)
			continue
		}
		return roots, nil
	}
	return nil, fmt.Errorf("signing: no Fulcio root certificate in Sigstore TUF targets: %v", lastErr)
}
