package verify

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"agentattest.dev/agentattest/internal/contracts"
)

type goldenWant struct {
	Valid        bool     `json:"valid"`
	Level        string   `json:"level"`
	FailureCodes []string `json:"failureCodes"`
}

func TestGoldenFixtures(t *testing.T) {
	files, err := contracts.Locate("")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(files.Root, "tests", "golden")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			statementJSON, err := os.ReadFile(filepath.Join(dir, "statement.json"))
			if err != nil {
				t.Fatal(err)
			}
			contextJSON, err := os.ReadFile(filepath.Join(dir, "context.json"))
			if err != nil {
				t.Fatal(err)
			}
			var want goldenWant
			readJSON(t, filepath.Join(dir, "want.json"), &want)
			if !want.Valid && len(want.FailureCodes) != 1 {
				t.Fatalf("invalid golden fixture must expect exactly one failure code, got %v", want.FailureCodes)
			}

			got := Predicate(context.Background(), statementJSON, contextJSON, Options{Contracts: files})
			if got.Valid != want.Valid {
				t.Fatalf("valid = %v, want %v; failures=%v", got.Valid, want.Valid, got.FailureCodes)
			}
			if got.Level != want.Level {
				t.Fatalf("level = %q, want %q", got.Level, want.Level)
			}
			if !reflect.DeepEqual(got.FailureCodes, want.FailureCodes) {
				t.Fatalf("failureCodes = %v, want %v", got.FailureCodes, want.FailureCodes)
			}
		})
	}
}
