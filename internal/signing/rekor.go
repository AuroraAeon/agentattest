package signing

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/transparency-dev/merkle/proof"
)

// RekorInclusion is a Rekor transparency-log inclusion proof for one entry.
type RekorInclusion struct {
	LogIndex uint64
	TreeSize uint64
	RootHash []byte
	Proof    [][]byte
	LeafData []byte // the canonicalized Rekor entry, hashed as an RFC 6962 leaf
}

// rfc6962Hasher implements merkle.LogHasher using the RFC 6962 SHA-256
// construction: leaves are H(0x00 || data), interior nodes H(0x01 || l || r).
type rfc6962Hasher struct{}

func (rfc6962Hasher) EmptyRoot() []byte {
	sum := sha256.Sum256(nil)
	return sum[:]
}

func (rfc6962Hasher) HashLeaf(leaf []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(leaf)
	return h.Sum(nil)
}

func (rfc6962Hasher) HashChildren(l, r []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(l)
	h.Write(r)
	return h.Sum(nil)
}

func (rfc6962Hasher) Size() int { return sha256.Size }

// VerifyInclusion verifies that LeafData is committed to by RootHash at LogIndex
// within a Merkle tree of TreeSize, per RFC 6962 (transparency-dev/merkle).
func VerifyInclusion(inc RekorInclusion) error {
	hasher := rfc6962Hasher{}
	leafHash := hasher.HashLeaf(inc.LeafData)
	return proof.VerifyInclusion(hasher, inc.LogIndex, inc.TreeSize, leafHash, inc.Proof, inc.RootHash)
}

// verifyRekorInclusion enforces the optional Rekor binding on a parsed bundle.
func verifyRekorInclusion(root any, trust TrustRoot) error {
	inc, ok := extractRekorInclusion(root)
	if !ok {
		if trust.RequireInclusion {
			return errors.New("signing: bundle has no transparency-log inclusion proof")
		}
		return nil
	}
	if err := VerifyInclusion(inc); err != nil {
		return fmt.Errorf("signing: rekor inclusion proof invalid: %w", err)
	}
	if len(trust.RekorRootHash) > 0 && !bytes.Equal(inc.RootHash, trust.RekorRootHash) {
		return errors.New("signing: rekor root hash does not match the trusted log root")
	}
	return nil
}

// extractRekorInclusion pulls the first tlogEntry's inclusion proof and
// canonicalized body out of a parsed Sigstore bundle.
func extractRekorInclusion(root any) (RekorInclusion, bool) {
	entries, ok := findKey(root, "tlogEntries")
	if !ok {
		return RekorInclusion{}, false
	}
	arr, ok := entries.([]any)
	if !ok || len(arr) == 0 {
		return RekorInclusion{}, false
	}
	entry, _ := arr[0].(map[string]any)
	incRaw, ok := entry["inclusionProof"].(map[string]any)
	if !ok {
		return RekorInclusion{}, false
	}
	inc := RekorInclusion{
		LogIndex: parseUint(incRaw["logIndex"]),
		TreeSize: parseUint(incRaw["treeSize"]),
		RootHash: decodeB64(incRaw["rootHash"]),
		LeafData: decodeB64(entry["canonicalizedBody"]),
	}
	if hashes, ok := incRaw["hashes"].([]any); ok {
		for _, h := range hashes {
			inc.Proof = append(inc.Proof, decodeB64(h))
		}
	}
	return inc, true
}

func parseUint(v any) uint64 {
	switch n := v.(type) {
	case float64:
		return uint64(n)
	case json.Number:
		i, _ := n.Int64()
		return uint64(i)
	case string:
		var u uint64
		for _, c := range n {
			if c < '0' || c > '9' {
				return u
			}
			u = u*10 + uint64(c-'0')
		}
		return u
	default:
		return 0
	}
}

func decodeB64(v any) []byte {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b
	}
	return nil
}
