package signing

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// FromSigstoreBundle extracts the DSSE envelope and its signing certificate
// chain from a Sigstore bundle — the JSON emitted by cosign, actions/attest, and
// `gh attestation verify --format json` / `gh attestation download` — verifies
// the leaf certificate against the trust root, and returns the verified identity
// context plus the decoded statement payload.
//
// It re-verifies the certificate chain and DSSE signature itself (via
// VerifyWithTrustRoot), so the returned VerifiedContext is backed by the
// supplied trust root rather than by anything self-asserted. When the bundle
// carries a tlogEntry, its RFC 6962 inclusion proof is verified and — if the
// trust root pins a Rekor root hash — bound to that log root; with
// TrustRoot.RequireInclusion a bundle without an inclusion proof is rejected.
//
// For output containing multiple attestations, the first dsseEnvelope with a
// paired certificate chain is used; split multi-attestation output for full
// coverage.
func FromSigstoreBundle(ctx context.Context, bundleJSON []byte, trust TrustRoot) (VerifiedContext, []byte, error) {
	var root any
	if err := json.Unmarshal(bundleJSON, &root); err != nil {
		return VerifiedContext{}, nil, fmt.Errorf("signing: parse bundle: %w", err)
	}

	envRaw, certsRaw, ok := findBundle(root)
	if !ok {
		return VerifiedContext{}, nil, errors.New("signing: no dsseEnvelope found in bundle")
	}
	env, err := parseEnvelope(envRaw)
	if err != nil {
		return VerifiedContext{}, nil, err
	}
	certs, err := parseCertArray(certsRaw)
	if err != nil {
		return VerifiedContext{}, nil, err
	}
	if len(certs) == 0 {
		return VerifiedContext{}, nil, errors.New("signing: no certificate chain paired with the dsseEnvelope")
	}

	vc, err := VerifyWithTrustRoot(ctx, env, certs[0], certs[1:], trust)
	if err != nil {
		return VerifiedContext{}, nil, err
	}
	if err := verifyRekorInclusion(root, trust); err != nil {
		return VerifiedContext{}, nil, err
	}
	payload, err := env.DecodeB64Payload()
	if err != nil {
		return VerifiedContext{}, nil, fmt.Errorf("signing: decode payload: %w", err)
	}
	return vc, payload, nil
}

// findBundle locates a map that carries a dsseEnvelope and returns it together
// with the certificate chain found within that same map, keeping the two paired.
func findBundle(v any) (envelope, certificates any, ok bool) {
	switch t := v.(type) {
	case map[string]any:
		if env, has := t["dsseEnvelope"]; has {
			certs, _ := findKey(t, "certificates")
			return env, certs, true
		}
		for _, child := range t {
			if env, certs, found := findBundle(child); found {
				return env, certs, true
			}
		}
	case []any:
		for _, item := range t {
			if env, certs, found := findBundle(item); found {
				return env, certs, true
			}
		}
	}
	return nil, nil, false
}

// findKey returns the first value stored under key anywhere in the structure.
func findKey(v any, key string) (any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if val, ok := t[key]; ok {
			return val, true
		}
		for _, child := range t {
			if found, ok := findKey(child, key); ok {
				return found, true
			}
		}
	case []any:
		for _, item := range t {
			if found, ok := findKey(item, key); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func parseEnvelope(v any) (*dsse.Envelope, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("signing: dsseEnvelope is not an object")
	}
	payloadType, _ := m["payloadType"].(string)
	payload, _ := m["payload"].(string)
	env := &dsse.Envelope{PayloadType: payloadType, Payload: payload}
	if sigs, ok := m["signatures"].([]any); ok {
		for _, s := range sigs {
			sm, _ := s.(map[string]any)
			keyID, _ := sm["keyid"].(string)
			sig, _ := sm["sig"].(string)
			env.Signatures = append(env.Signatures, dsse.Signature{KeyID: keyID, Sig: sig})
		}
	}
	return env, nil
}

func parseCertArray(v any) ([]*x509.Certificate, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, nil // absent or empty chain is handled by the caller
	}
	var certs []*x509.Certificate
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			continue
		}
		cert, err := parseCertString(s)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// parseCertString accepts a base64-encoded DER certificate (the Sigstore bundle
// form) or a PEM certificate.
func parseCertString(s string) (*x509.Certificate, error) {
	if der, err := base64.StdEncoding.DecodeString(s); err == nil {
		if cert, err := x509.ParseCertificate(der); err == nil {
			return cert, nil
		}
	}
	if certs, err := ParseCertificates([]byte(s)); err == nil && len(certs) > 0 {
		return certs[0], nil
	}
	return nil, fmt.Errorf("signing: could not decode certificate value")
}
