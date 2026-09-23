package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyBundleRequiresBundleFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runVerifyBundle(t.Context(), nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--bundle is required") {
		t.Fatalf("stderr = %q, want mention of --bundle", stderr.String())
	}
}

func TestVerifyBundleBadTrustRootEmitsSignatureInvalid(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(bundlePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	trustPath := filepath.Join(dir, "root.pem")
	if err := os.WriteFile(trustPath, []byte("not a certificate"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runVerifyBundle(t.Context(), []string{"--bundle", bundlePath, "--trust-root", trustPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	var result struct {
		Valid        bool     `json:"valid"`
		FailureCodes []string `json:"failureCodes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not a structured result (%q): %v", stdout.String(), err)
	}
	if result.Valid || len(result.FailureCodes) != 1 || result.FailureCodes[0] != "signature_invalid" {
		t.Fatalf("result = %+v, want valid=false with [signature_invalid]", result)
	}
}

// TestVerifyBundleEmptyTrustRootFailsClosed pins that an explicitly supplied
// but empty --trust-root file never falls back to the TUF-sourced public root.
func TestVerifyBundleEmptyTrustRootFailsClosed(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(bundlePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	trustPath := filepath.Join(dir, "root.pem")
	if err := os.WriteFile(trustPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runVerifyBundle(t.Context(), []string{"--bundle", bundlePath, "--trust-root", trustPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "is empty") {
		t.Fatalf("stderr = %q, want an empty-trust-root error", stderr.String())
	}
	var result struct {
		Valid        bool     `json:"valid"`
		FailureCodes []string `json:"failureCodes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not a structured result (%q): %v", stdout.String(), err)
	}
	if result.Valid || len(result.FailureCodes) != 1 || result.FailureCodes[0] != "signature_invalid" {
		t.Fatalf("result = %+v, want valid=false with [signature_invalid]", result)
	}
}
