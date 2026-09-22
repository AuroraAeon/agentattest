// Package registry is an optional, read-only discovery index over emitted
// agentattest statements. It maps subject digest, repository URL, and run ID to
// statement references so a consumer can find the attestation(s) for a change.
//
// It is a convenience index only: it stores references and non-raw metadata,
// never verification pass/fail decisions, and is never a trust root. All trust
// comes from re-verifying the referenced statement. Use is opt-in.
package registry

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultDBName is the default registry database file name.
const DefaultDBName = "registry.db"

// Entry is one indexed statement reference.
type Entry struct {
	SubjectDigest string `json:"subjectDigest"`
	RepoURL       string `json:"repoUrl,omitempty"`
	RunID         string `json:"runId,omitempty"`
	PredicateType string `json:"predicateType,omitempty"`
	Level         string `json:"level,omitempty"`
	StatementPath string `json:"statementPath,omitempty"`
	IndexedAt     string `json:"indexedAt,omitempty"`
}

// Registry is a discovery index backed by SQLite.
type Registry struct {
	db *sql.DB
}

// Open opens (creating if needed) a registry database at path.
func Open(path string) (*Registry, error) {
	if path == "" {
		return nil, fmt.Errorf("registry: empty path")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Registry{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`create table if not exists statements (
		subject_digest text not null,
		repo_url text,
		run_id text,
		predicate_type text,
		level text,
		statement_path text,
		indexed_at text,
		primary key (subject_digest, statement_path)
	)`)
	return err
}

// Add indexes a statement by each of its SHA-256 subject digests. statementPath
// is a reference (path or URI) to the statement; the statement bytes themselves
// are not stored. Add is idempotent.
func (r *Registry) Add(ctx context.Context, statementJSON []byte, statementPath string) error {
	var doc struct {
		PredicateType string `json:"predicateType"`
		Subject       []struct {
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
		Predicate struct {
			RunID string `json:"runId"`
			Level string `json:"verificationLevel"`
			Repo  struct {
				URL string `json:"url"`
			} `json:"repo"`
		} `json:"predicate"`
	}
	dec := json.NewDecoder(bytes.NewReader(statementJSON))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return fmt.Errorf("registry: decode statement: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	seen := make(map[string]bool)
	for _, s := range doc.Subject {
		digest := s.Digest["sha256"]
		if digest == "" || seen[digest] {
			continue
		}
		seen[digest] = true
		if _, err := r.db.ExecContext(ctx,
			`insert or replace into statements(subject_digest, repo_url, run_id, predicate_type, level, statement_path, indexed_at) values (?, ?, ?, ?, ?, ?, ?)`,
			digest, nullable(doc.Predicate.Repo.URL), nullable(doc.Predicate.RunID), nullable(doc.PredicateType), nullable(doc.Predicate.Level), nullable(statementPath), now,
		); err != nil {
			return err
		}
	}
	return nil
}

// BySubject returns entries indexed under a subject digest.
func (r *Registry) BySubject(ctx context.Context, digest string) ([]Entry, error) {
	return r.query(ctx, "where subject_digest = ?", digest)
}

// ByRepo returns entries for a repository URL.
func (r *Registry) ByRepo(ctx context.Context, repoURL string) ([]Entry, error) {
	return r.query(ctx, "where repo_url = ?", repoURL)
}

// ByRunID returns entries for a run ID.
func (r *Registry) ByRunID(ctx context.Context, runID string) ([]Entry, error) {
	return r.query(ctx, "where run_id = ?", runID)
}

const selectColumns = `select subject_digest, coalesce(repo_url,''), coalesce(run_id,''), coalesce(predicate_type,''), coalesce(level,''), coalesce(statement_path,''), coalesce(indexed_at,'') from statements`

func (r *Registry) query(ctx context.Context, clause string, arg any) ([]Entry, error) {
	rows, err := r.db.QueryContext(ctx, selectColumns+" "+clause+" order by indexed_at, subject_digest", arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.SubjectDigest, &e.RepoURL, &e.RunID, &e.PredicateType, &e.Level, &e.StatementPath, &e.IndexedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Close closes the underlying database.
func (r *Registry) Close() error {
	return r.db.Close()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
