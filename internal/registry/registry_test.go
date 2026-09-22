package registry

import (
	"context"
	"path/filepath"
	"testing"
)

const sampleStatement = `{
  "_type": "https://in-toto.io/Statement/v1",
  "subject": [
    { "name": "patch.diff", "digest": { "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" } },
    { "name": "repo-tree", "digest": { "sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" } }
  ],
  "predicateType": "https://agentattest.dev/predicate/v1",
  "predicate": {
    "predicateVersion": "v1",
    "runId": "run_01HTYJ7B4A3H8M0S8P7M2N9Q",
    "verificationLevel": "policy-grade",
    "repo": { "url": "https://github.com/example/agentattest", "baseCommit": "0123456789abcdef0123456789abcdef01234567" }
  }
}`

func openTemp(t *testing.T) *Registry {
	t.Helper()
	reg, err := Open(filepath.Join(t.TempDir(), DefaultDBName))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}

func TestRegistryAddAndQuery(t *testing.T) {
	ctx := context.Background()
	reg := openTemp(t)

	if err := reg.Add(ctx, []byte(sampleStatement), "statements/run_01.json"); err != nil {
		t.Fatalf("add: %v", err)
	}

	bySubject, err := reg.BySubject(ctx, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("by subject: %v", err)
	}
	if len(bySubject) != 1 {
		t.Fatalf("by subject returned %d entries", len(bySubject))
	}
	e := bySubject[0]
	if e.RepoURL != "https://github.com/example/agentattest" || e.RunID != "run_01HTYJ7B4A3H8M0S8P7M2N9Q" {
		t.Fatalf("entry = %+v", e)
	}
	if e.PredicateType != "https://agentattest.dev/predicate/v1" || e.Level != "policy-grade" {
		t.Fatalf("entry metadata = %+v", e)
	}

	// The second subject of the same statement is also indexed.
	byTree, err := reg.BySubject(ctx, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil || len(byTree) != 1 {
		t.Fatalf("by repo-tree subject: %v (%d)", err, len(byTree))
	}

	byRepo, err := reg.ByRepo(ctx, "https://github.com/example/agentattest")
	if err != nil || len(byRepo) != 2 { // one row per subject digest
		t.Fatalf("by repo: %v (%d entries)", err, len(byRepo))
	}

	byRun, err := reg.ByRunID(ctx, "run_01HTYJ7B4A3H8M0S8P7M2N9Q")
	if err != nil || len(byRun) != 2 {
		t.Fatalf("by run: %v (%d entries)", err, len(byRun))
	}
}

func TestRegistryAddIsIdempotent(t *testing.T) {
	ctx := context.Background()
	reg := openTemp(t)
	if err := reg.Add(ctx, []byte(sampleStatement), "statements/run_01.json"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Add(ctx, []byte(sampleStatement), "statements/run_01.json"); err != nil {
		t.Fatal(err)
	}
	byRun, err := reg.ByRunID(ctx, "run_01HTYJ7B4A3H8M0S8P7M2N9Q")
	if err != nil {
		t.Fatal(err)
	}
	if len(byRun) != 2 {
		t.Fatalf("idempotent add changed row count: %d", len(byRun))
	}
}

func TestRegistryQueryMissIsEmpty(t *testing.T) {
	ctx := context.Background()
	reg := openTemp(t)
	got, err := reg.BySubject(ctx, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no entries for unknown digest, got %d", len(got))
	}
}
