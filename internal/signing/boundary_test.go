package signing

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestCryptoImportsConfinedToSigning machine-enforces the ARCHITECTURE.md
// boundary: only internal/signing may import DSSE, Merkle-proof, X.509, or
// asymmetric-crypto packages. Digest hashing (crypto/sha256) is allowed
// everywhere per ARCHITECTURE.md. The rule previously existed only as prose.
func TestCryptoImportsConfinedToSigning(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	forbidden := []string{
		"github.com/secure-systems-lab/go-securesystemslib/dsse",
		"github.com/transparency-dev/merkle",
		"crypto/ecdsa",
		"crypto/ed25519",
		"crypto/rsa",
		"crypto/x509",
		"encoding/pem",
	}
	allowedAnywhere := map[string]bool{
		"crypto/sha256": true,
		"crypto/sha512": true,
	}

	var violations []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "dist", "node_modules", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		inSigning := strings.HasPrefix(filepath.ToSlash(rel), "internal/signing/")

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if allowedAnywhere[importPath] {
				continue
			}
			for _, bad := range forbidden {
				if importPath == bad || strings.HasPrefix(importPath, bad+"/") {
					if !inSigning {
						violations = append(violations, filepath.ToSlash(rel)+": "+importPath)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("crypto/signing imports outside internal/signing (ARCHITECTURE.md boundary):\n  %s",
			strings.Join(violations, "\n  "))
	}
}
