package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type orderingStatement struct {
	Subject []struct {
		Name   string            `json:"name"`
		Digest map[string]string `json:"digest"`
	} `json:"subject"`
	Predicate struct {
		PredicateVersion string `json:"predicateVersion"`
	} `json:"predicate"`
}

type orderingContext struct {
	Subjects []struct {
		Name      string `json:"name"`
		Algorithm string `json:"algorithm"`
		Digest    string `json:"digest"`
	} `json:"subjects"`
}

type orderingWant struct {
	Statement    orderingStatement `json:"statement"`
	Context      orderingContext   `json:"context"`
	FailureCodes []string          `json:"failureCodes"`
}

func TestFailureOrderingFixture(t *testing.T) {
	var want orderingWant
	readJSON(t, filepath.Join("testdata", "unsupported-version-subject-mismatch.json"), &want)
	if len(want.FailureCodes) == 0 {
		t.Fatal("ordering fixture must include at least one failure code")
	}

	codes := detectOrderingFixtureFailures(want.Statement, want.Context)
	ordered := OrderFailureCodes(codes)
	if len(ordered) == 0 {
		t.Fatal("expected detected failure codes")
	}
	if !reflect.DeepEqual(ordered, want.FailureCodes) {
		t.Fatalf("ordered failure codes = %v, want %v", ordered, want.FailureCodes)
	}
}

func detectOrderingFixtureFailures(statement orderingStatement, context orderingContext) []string {
	var codes []string
	if statement.Predicate.PredicateVersion != "v0" {
		codes = append(codes, "unsupported_predicate_version")
	}
	if !subjectsMatch(statement.Subject, context.Subjects) {
		codes = append(codes, "subject_digest_mismatch")
	}
	return codes
}

func subjectsMatch(statementSubjects []struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}, contextSubjects []struct {
	Name      string `json:"name"`
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}) bool {
	if len(statementSubjects) != len(contextSubjects) {
		return false
	}

	for _, expected := range contextSubjects {
		found := false
		for _, subject := range statementSubjects {
			if subject.Name == expected.Name && subject.Digest[expected.Algorithm] == expected.Digest {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(content, target); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
