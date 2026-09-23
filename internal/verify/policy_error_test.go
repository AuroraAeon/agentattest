package verify

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agentattest.dev/agentattest/internal/contracts"
)

// TestPolicyEvalErrorOnBrokenPolicy pins the stable policy_eval_error code:
// when the Rego policy module cannot be compiled/evaluated the verifier must
// fail closed with exactly that code (not a panic, not an empty result).
func TestPolicyEvalErrorOnBrokenPolicy(t *testing.T) {
	files, err := contracts.Locate("")
	if err != nil {
		t.Fatalf("locate contracts: %v", err)
	}
	files.DefaultPolicy = filepath.Join("testdata", "bad-policy.rego")

	statementJSON, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "valid-minimal", "statement.json"))
	if err != nil {
		t.Fatalf("read fixture statement: %v", err)
	}
	contextJSON, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "valid-minimal", "context.json"))
	if err != nil {
		t.Fatalf("read fixture context: %v", err)
	}

	result := Predicate(context.Background(), statementJSON, contextJSON, Options{Contracts: files})
	if result.Valid {
		t.Fatal("broken policy must not verify")
	}
	if !reflect.DeepEqual(result.FailureCodes, []string{"policy_eval_error"}) {
		t.Fatalf("failure codes = %v, want [policy_eval_error]", result.FailureCodes)
	}
}

// TestGoldenFixturesLocateContractPack guards the fixture path used above.
func TestGoldenFixturesLocateContractPack(t *testing.T) {
	if _, err := contracts.Locate(""); err != nil {
		t.Fatalf("locate contracts from verify package: %v", err)
	}
}
