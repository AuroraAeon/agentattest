package contracts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	PredicateSchemaRel   = "schemas/agent-provenance-v0.schema.json"
	PredicateCUERel      = "schemas/agent-provenance-v0.cue"
	PredicateSchemaV1Rel = "schemas/agent-provenance-v1.schema.json"
	PredicateCUEV1Rel    = "schemas/agent-provenance-v1.cue"
	DefaultPolicyRel     = "policies/default.rego"
	ContextSchemaRel     = "tests/golden/context.schema.json"
)

type Files struct {
	Root              string
	PredicateSchema   string
	PredicateCUE      string
	PredicateSchemaV1 string
	PredicateCUEV1    string
	DefaultPolicy     string
	ContextSchema     string
}

func Locate(start string) (Files, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return Files{}, err
		}
	}
	start, err := filepath.Abs(start)
	if err != nil {
		return Files{}, err
	}
	info, err := os.Stat(start)
	if err != nil {
		return Files{}, err
	}
	if !info.IsDir() {
		start = filepath.Dir(start)
	}

	for dir := start; ; dir = filepath.Dir(dir) {
		files := Files{
			Root:              dir,
			PredicateSchema:   filepath.Join(dir, filepath.FromSlash(PredicateSchemaRel)),
			PredicateCUE:      filepath.Join(dir, filepath.FromSlash(PredicateCUERel)),
			PredicateSchemaV1: filepath.Join(dir, filepath.FromSlash(PredicateSchemaV1Rel)),
			PredicateCUEV1:    filepath.Join(dir, filepath.FromSlash(PredicateCUEV1Rel)),
			DefaultPolicy:     filepath.Join(dir, filepath.FromSlash(DefaultPolicyRel)),
			ContextSchema:     filepath.Join(dir, filepath.FromSlash(ContextSchemaRel)),
		}
		if files.exist() {
			return files, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return Files{}, fmt.Errorf("could not locate agentattest contract files from %s", start)
}

// exist reports whether the minimum v0 contract pack is present. v1 contract
// files are located alongside but are not required to locate the pack, so a
// v0-only checkout still works; verifying a v1 statement against a tree that
// lacks the v1 contract files fails closed with schema_invalid.
func (f Files) exist() bool {
	for _, path := range []string{f.PredicateSchema, f.PredicateCUE, f.DefaultPolicy, f.ContextSchema} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func Read(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("contract path is empty")
	}
	return os.ReadFile(path)
}
