package signing

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/transparency-dev/merkle/rfc6962"
)

func TestVerifyInclusionValid(t *testing.T) {
	leaf := []byte("canonical-rekor-entry")
	hasher := rfc6962.DefaultHasher
	root := hasher.HashLeaf(leaf) // a size-1 tree's root is the leaf hash
	if err := VerifyInclusion(RekorInclusion{LogIndex: 0, TreeSize: 1, RootHash: root, LeafData: leaf}); err != nil {
		t.Fatalf("valid inclusion proof rejected: %v", err)
	}
}

func TestVerifyInclusionRejectsTamperedLeaf(t *testing.T) {
	leaf := []byte("canonical-rekor-entry")
	hasher := rfc6962.DefaultHasher
	root := hasher.HashLeaf(leaf)
	if err := VerifyInclusion(RekorInclusion{LogIndex: 0, TreeSize: 1, RootHash: root, LeafData: []byte("different")}); err == nil {
		t.Fatal("expected inclusion proof to reject a mismatched leaf")
	}
}

// bundleWithInclusion injects a valid size-1 Rekor inclusion proof into a bundle.
func bundleWithInclusion(t *testing.T, leafData []byte) (bundleJSON, rootHash []byte, root *x509.Certificate) {
	t.Helper()
	base, root, _ := makeBundle(t, []byte(`{"_type":"https://in-toto.io/Statement/v1"}`))
	var obj map[string]any
	if err := json.Unmarshal(base, &obj); err != nil {
		t.Fatal(err)
	}
	rootHash = (rfc6962.DefaultHasher).HashLeaf(leafData)
	material := obj["verificationMaterial"].(map[string]any)
	material["tlogEntries"] = []map[string]any{
		{
			"logIndex": float64(0),
			"inclusionProof": map[string]any{
				"logIndex": float64(0),
				"treeSize": float64(1),
				"rootHash": base64.StdEncoding.EncodeToString(rootHash),
				"hashes":   []string{},
			},
			"canonicalizedBody": base64.StdEncoding.EncodeToString(leafData),
		},
	}
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return out, rootHash, root
}

func TestFromSigstoreBundleBindsRekorRoot(t *testing.T) {
	ctx := context.Background()
	bundleJSON, rootHash, root := bundleWithInclusion(t, []byte("rekor-entry-body"))

	if _, _, err := FromSigstoreBundle(ctx, bundleJSON, TrustRoot{Roots: []*x509.Certificate{root}, RekorRootHash: rootHash}); err != nil {
		t.Fatalf("expected success with matching rekor root: %v", err)
	}

	wrong := append([]byte(nil), rootHash...)
	wrong[0] ^= 0xff
	if _, _, err := FromSigstoreBundle(ctx, bundleJSON, TrustRoot{Roots: []*x509.Certificate{root}, RekorRootHash: wrong}); err == nil {
		t.Fatal("expected failure with a mismatched rekor root")
	}
}

func TestFromSigstoreBundleRequireInclusion(t *testing.T) {
	ctx := context.Background()
	bundleJSON, root, _ := makeBundle(t, []byte(`{"_type":"https://in-toto.io/Statement/v1"}`))

	if _, _, err := FromSigstoreBundle(ctx, bundleJSON, TrustRoot{Roots: []*x509.Certificate{root}, RequireInclusion: true}); err == nil {
		t.Fatal("expected failure when inclusion is required but the bundle has none")
	}
	if _, _, err := FromSigstoreBundle(ctx, bundleJSON, TrustRoot{Roots: []*x509.Certificate{root}}); err != nil {
		t.Fatalf("expected success without an inclusion requirement: %v", err)
	}
}
