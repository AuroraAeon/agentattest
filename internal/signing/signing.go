// Package signing isolates all DSSE signing and verification. It is the only
// package permitted to import DSSE / signing libraries (see ARCHITECTURE.md).
//
// It has two jobs and nothing else:
//
//   - Sign wraps an in-toto Statement payload in a DSSE envelope using a
//     caller-supplied crypto.Signer (no custom crypto, no key management).
//   - Verify verifies a DSSE envelope against trusted signer certificates and
//     returns the out-of-band VerifiedContext the default policy consumes
//     (verifiedSigner / verifiedBuilderId / verifiedWorkflowRef / verifiedIssuer).
//
// The DSSE envelope and its pre-authentication encoding come from
// secure-systems-lab/go-securesystemslib; signature math uses Go's standard
// library. This adapter never computes git digests, mutates predicates, or
// decides verification success — it only establishes "who signed this payload".
//
// Scope note: this is the envelope + identity layer. Full chain-to-Fulcio-root
// and Rekor inclusion-proof verification is a later layer that feeds trusted
// certificates in; the identity extraction here already matches the Sigstore
// keyless model (SAN for the identity, Fulcio OIDC extension for the issuer).
package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// PayloadType is the DSSE payload type for in-toto attestations.
const PayloadType = "application/vnd.in-toto+json"

// fulcioIssuerOID is the OIDC issuer extension Sigstore/Fulcio embeds in keyless
// signing certificates (1.3.6.1.4.1.57264.1.1).
const fulcioIssuerOID = "1.3.6.1.4.1.57264.1.1"

// VerifiedContext is the identity subset of verifier context derivable from a
// verified signature. The caller merges it with repo/base/subject context before
// invoking the verifier. Keys map 1:1 to tests/golden/context.schema.json.
type VerifiedContext struct {
	VerifiedSigner      string
	VerifiedBuilderID   string
	VerifiedWorkflowRef string
	VerifiedIssuer      string
}

// ToContextMap returns the non-empty identity fields as verifier-context keys so
// the result can be merged into a larger context object.
func (vc VerifiedContext) ToContextMap() map[string]any {
	out := map[string]any{}
	if vc.VerifiedSigner != "" {
		out["verifiedSigner"] = vc.VerifiedSigner
	}
	if vc.VerifiedBuilderID != "" {
		out["verifiedBuilderId"] = vc.VerifiedBuilderID
	}
	if vc.VerifiedWorkflowRef != "" {
		out["verifiedWorkflowRef"] = vc.VerifiedWorkflowRef
	}
	if vc.VerifiedIssuer != "" {
		out["verifiedIssuer"] = vc.VerifiedIssuer
	}
	return out
}

// Sign wraps payload in a DSSE envelope signed by key. An empty payloadType
// defaults to the in-toto media type.
func Sign(ctx context.Context, payloadType string, payload []byte, key crypto.Signer) (*dsse.Envelope, error) {
	if key == nil {
		return nil, errors.New("signing: nil key")
	}
	if payloadType == "" {
		payloadType = PayloadType
	}
	signer, err := dsse.NewEnvelopeSigner(&cryptoSigner{key: key})
	if err != nil {
		return nil, fmt.Errorf("signing: envelope signer: %w", err)
	}
	return signer.SignPayload(ctx, payloadType, payload)
}

// Verify verifies a DSSE envelope against the trusted signer certificates and
// returns the verified identity context. It fails closed on a nil envelope, no
// trusted certificates, a missing signature, or a signature that matches none of
// the trusted certificates.
//
// trustedCerts are the leaf signer certificates to accept — e.g. extracted from
// a Sigstore/cosign bundle or pinned by repository policy.
func Verify(ctx context.Context, env *dsse.Envelope, trustedCerts []*x509.Certificate) (VerifiedContext, error) {
	if env == nil {
		return VerifiedContext{}, errors.New("signing: nil envelope")
	}
	if len(trustedCerts) == 0 {
		return VerifiedContext{}, errors.New("signing: no trusted certificates")
	}

	verifiers := make([]dsse.Verifier, 0, len(trustedCerts))
	for _, cert := range trustedCerts {
		verifiers = append(verifiers, &certVerifier{cert: cert})
	}
	ev, err := dsse.NewEnvelopeVerifier(verifiers...)
	if err != nil {
		return VerifiedContext{}, fmt.Errorf("signing: envelope verifier: %w", err)
	}
	if _, _, err := ev.VerifyAndDecode(ctx, env); err != nil {
		return VerifiedContext{}, fmt.Errorf("signing: verify envelope: %w", err)
	}

	// EnvelopeVerifier calls Verifier.Verify per candidate; the ones that
	// succeeded set verified=true. Use the first verified cert's identity.
	for _, v := range verifiers {
		if cv, ok := v.(*certVerifier); ok && cv.verified {
			return identityFromCert(cv.cert), nil
		}
	}
	return VerifiedContext{}, errors.New("signing: no accepted signature matched a trusted certificate")
}

// cryptoSigner adapts a crypto.Signer to the DSSE Signer interface. DSSE hands
// it the pre-authentication encoding; it hashes and signs per key type.
type cryptoSigner struct {
	key crypto.Signer
}

func (s *cryptoSigner) Sign(_ context.Context, data []byte) ([]byte, error) {
	switch s.key.Public().(type) {
	case *ecdsa.PublicKey, *rsa.PublicKey:
		digest := sha256.Sum256(data)
		return s.key.Sign(rand.Reader, digest[:], crypto.SHA256)
	case ed25519.PublicKey:
		return s.key.Sign(rand.Reader, data, crypto.Hash(0))
	default:
		return nil, fmt.Errorf("signing: unsupported key type %T", s.key.Public())
	}
}

func (s *cryptoSigner) KeyID() (string, error) {
	return dsse.SHA256KeyID(s.key.Public())
}

// certVerifier verifies DSSE signatures against a certificate's public key and
// records whether it produced an accepted signature.
type certVerifier struct {
	cert     *x509.Certificate
	verified bool
}

func (v *certVerifier) KeyID() (string, error) {
	return dsse.SHA256KeyID(v.cert.PublicKey)
}

func (v *certVerifier) Public() crypto.PublicKey {
	return v.cert.PublicKey
}

func (v *certVerifier) Verify(_ context.Context, data, sig []byte) error {
	digest := sha256.Sum256(data)
	switch pub := v.cert.PublicKey.(type) {
	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(pub, digest[:], sig) {
			return errors.New("signing: ecdsa signature verification failed")
		}
	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
			return fmt.Errorf("signing: rsa signature verification failed: %w", err)
		}
	case ed25519.PublicKey:
		if !ed25519.Verify(pub, data, sig) {
			return errors.New("signing: ed25519 signature verification failed")
		}
	default:
		return fmt.Errorf("signing: unsupported key type %T", pub)
	}
	v.verified = true
	return nil
}

// identityFromCert maps a signer certificate to verifier context using the
// Sigstore keyless identity model: a URI SAN (workload identity, e.g. a GitHub
// Actions workflow) also fills builder/workflow; otherwise an email or DNS SAN;
// the Fulcio OIDC extension supplies the issuer.
func fulcioIssuer(cert *x509.Certificate) string {
	for _, ext := range cert.Extensions {
		if ext.Id.String() == fulcioIssuerOID {
			return strings.TrimSpace(string(ext.Value))
		}
	}
	return ""
}

func identityFromCert(cert *x509.Certificate) VerifiedContext {
	vc := VerifiedContext{VerifiedIssuer: fulcioIssuer(cert)}
	switch {
	case len(cert.URIs) > 0:
		vc.VerifiedSigner = cert.URIs[0].String()
		if isGitHubWorkflowRef(vc.VerifiedSigner) {
			vc.VerifiedWorkflowRef = vc.VerifiedSigner
			vc.VerifiedBuilderID = vc.VerifiedSigner
		}
	case len(cert.EmailAddresses) > 0:
		vc.VerifiedSigner = cert.EmailAddresses[0]
	case len(cert.DNSNames) > 0:
		vc.VerifiedSigner = cert.DNSNames[0]
	default:
		vc.VerifiedSigner = cert.Subject.CommonName
	}
	return vc
}

func isGitHubWorkflowRef(s string) bool {
	return strings.HasPrefix(s, "https://github.com/") && strings.Contains(s, "/.github/workflows/")
}

// TrustRoot configures which certificate authorities and OIDC issuers are
// accepted as verified signer identities. For the public-good Sigstore instance
// the roots are the Fulcio root CAs; for a private instance or tests they are
// the operator's own roots. AllowedIssuers, when non-empty, constrains the
// Fulcio OIDC issuer the leaf certificate must carry.
type TrustRoot struct {
	Roots          []*x509.Certificate
	AllowedIssuers []string

	// Rekor transparency-log binding (optional). When a bundle carries a Rekor
	// tlogEntries inclusion proof, FromSigstoreBundle verifies it per RFC 6962.
	// RequireInclusion fails closed if a bundle has no inclusion proof.
	// RekorRootHash, when set, additionally requires the proven root to equal a
	// known trusted log root. Verifying that the root hash is the authentic
	// Rekor-signed checkpoint (tlog-checkpoint format + the live Rekor key) is a
	// separate, caller-supplied step.
	RequireInclusion bool
	RekorRootHash    []byte
}

// VerifyWithTrustRoot verifies that the envelope's leaf certificate chains to a
// trusted root (and, if configured, carries an allowed OIDC issuer), then
// verifies the DSSE signature with that leaf and returns the verified identity.
// It fails closed on a broken chain, an untrusted root, a disallowed issuer, or
// a signature mismatch.
func VerifyWithTrustRoot(ctx context.Context, env *dsse.Envelope, leaf *x509.Certificate, intermediates []*x509.Certificate, trust TrustRoot) (VerifiedContext, error) {
	if env == nil {
		return VerifiedContext{}, errors.New("signing: nil envelope")
	}
	if leaf == nil {
		return VerifiedContext{}, errors.New("signing: nil leaf certificate")
	}
	if len(trust.Roots) == 0 {
		return VerifiedContext{}, errors.New("signing: empty trust root")
	}

	roots := x509.NewCertPool()
	for _, r := range trust.Roots {
		roots.AddCert(r)
	}
	interPool := x509.NewCertPool()
	for _, c := range intermediates {
		interPool.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: interPool,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}); err != nil {
		return VerifiedContext{}, fmt.Errorf("signing: certificate chain verification failed: %w", err)
	}

	if len(trust.AllowedIssuers) > 0 {
		issuer := fulcioIssuer(leaf)
		if issuer == "" || !slices.Contains(trust.AllowedIssuers, issuer) {
			return VerifiedContext{}, fmt.Errorf("signing: issuer %q is not in the trust root allowlist", issuer)
		}
	}

	return Verify(ctx, env, []*x509.Certificate{leaf})
}

// ParseCertificates decodes PEM-encoded certificates — the form cosign and
// `gh attestation` expose the signing certificate chain in.
func ParseCertificates(pemBytes []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("signing: parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, errors.New("signing: no certificates found in PEM input")
	}
	return certs, nil
}
