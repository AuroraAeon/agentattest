package verify

import (
	"reflect"
	"testing"

	"agentattest.dev/agentattest/internal/contracts"
	"agentattest.dev/agentattest/internal/statement"
)

// TestPhase01VersionRouting locks the pre-schema gate for both v0 and v1
// statements, including type/version consistency and unknown-type rejection.
func TestPhase01VersionRouting(t *testing.T) {
	cases := []struct {
		name         string
		doc          statement.Document
		predicateVer string
		wantCodes    []string
	}{
		{
			name:         "v0 consistent",
			doc:          statement.Document{Type: statement.TypeV1, PredicateType: statement.PredicateTypeV0},
			predicateVer: "v0",
			wantCodes:    nil,
		},
		{
			name:         "v1 consistent",
			doc:          statement.Document{Type: statement.TypeV1, PredicateType: statement.PredicateTypeV1},
			predicateVer: "v1",
			wantCodes:    nil,
		},
		{
			name:         "v1 type with v0 version",
			doc:          statement.Document{Type: statement.TypeV1, PredicateType: statement.PredicateTypeV1},
			predicateVer: "v0",
			wantCodes:    []string{"unsupported_predicate_version"},
		},
		{
			name:         "v0 type with v1 version",
			doc:          statement.Document{Type: statement.TypeV1, PredicateType: statement.PredicateTypeV0},
			predicateVer: "v1",
			wantCodes:    []string{"unsupported_predicate_version"},
		},
		{
			name:         "missing version",
			doc:          statement.Document{Type: statement.TypeV1, PredicateType: statement.PredicateTypeV1},
			predicateVer: "",
			wantCodes:    []string{"unsupported_predicate_version"},
		},
		{
			name:         "unknown predicate type",
			doc:          statement.Document{Type: statement.TypeV1, PredicateType: "https://example.org/other/v9"},
			predicateVer: "v9",
			wantCodes:    []string{"predicate_type_mismatch", "unsupported_predicate_version"},
		},
		{
			name:         "bad statement type",
			doc:          statement.Document{Type: "https://example.org/Statement/v0", PredicateType: statement.PredicateTypeV0},
			predicateVer: "v0",
			wantCodes:    []string{"unsupported_statement_type"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			predicate := map[string]any{}
			if tc.predicateVer != "" {
				predicate["predicateVersion"] = tc.predicateVer
			}
			got := phase01(tc.doc, predicate)
			if !reflect.DeepEqual(got, tc.wantCodes) {
				t.Fatalf("phase01 codes = %v, want %v", got, tc.wantCodes)
			}
		})
	}
}

func TestContractForVersion(t *testing.T) {
	files := contracts.Files{
		PredicateSchema:   "s0.json",
		PredicateCUE:      "c0.cue",
		PredicateSchemaV1: "s1.json",
		PredicateCUEV1:    "c1.cue",
	}

	schema, cue, def, ok := contractForVersion(files, statement.PredicateTypeV0)
	if !ok || schema != "s0.json" || cue != "c0.cue" || def != "#AgentProvenanceV0" {
		t.Fatalf("v0 routing = %q %q %q %v", schema, cue, def, ok)
	}

	schema, cue, def, ok = contractForVersion(files, statement.PredicateTypeV1)
	if !ok || schema != "s1.json" || cue != "c1.cue" || def != "#AgentProvenanceV1" {
		t.Fatalf("v1 routing = %q %q %q %v", schema, cue, def, ok)
	}

	if _, _, _, ok := contractForVersion(files, "https://example.org/unknown"); ok {
		t.Fatal("unknown predicateType must not route to a contract")
	}
}

func TestStatementNewV1(t *testing.T) {
	stmt := statement.NewV1(
		[]statement.Subject{{Name: "patch.diff", Digest: map[string]string{"sha256": "aa"}}},
		map[string]any{"predicateVersion": "v1"},
	)
	if stmt["_type"] != statement.TypeV1 {
		t.Fatalf("NewV1 _type = %v, want %v", stmt["_type"], statement.TypeV1)
	}
	if stmt["predicateType"] != statement.PredicateTypeV1 {
		t.Fatalf("NewV1 predicateType = %v, want %v", stmt["predicateType"], statement.PredicateTypeV1)
	}
}
