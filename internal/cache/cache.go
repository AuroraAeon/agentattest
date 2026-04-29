package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"agentattest.dev/agentattest/internal/gitbind"

	_ "modernc.org/sqlite"
)

const (
	DirName    = ".agentattest"
	DBFileName = "cache.db"
	ConfigName = "config.json"
)

type Config struct {
	PredicateType    string        `json:"predicateType"`
	PredicateVersion string        `json:"predicateVersion"`
	Cache            CacheConfig   `json:"cache"`
	Privacy          PrivacyConfig `json:"privacy"`
}

type CacheConfig struct {
	Path string `json:"path"`
}

type PrivacyConfig struct {
	RedactionPolicy string `json:"redactionPolicy"`
	RawPromptStored bool   `json:"rawPromptStored"`
	RawToolStored   bool   `json:"rawToolOutputsStored"`
}

func Init(ctx context.Context, repoDir string) (string, error) {
	dir := filepath.Join(repoDir, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dbPath := filepath.Join(dir, DBFileName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", err
	}
	defer db.Close()
	if err := migrate(ctx, db); err != nil {
		return "", err
	}

	configPath := filepath.Join(dir, ConfigName)
	config := Config{
		PredicateType:    "https://agentattest.dev/predicate/v0",
		PredicateVersion: "v0",
		Cache:            CacheConfig{Path: filepath.ToSlash(filepath.Join(DirName, DBFileName))},
		Privacy: PrivacyConfig{
			RedactionPolicy: "default-minimal-v0",
			RawPromptStored: false,
			RawToolStored:   false,
		},
	}
	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	content = append(content, '\n')
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		return "", err
	}
	return dbPath, nil
}

func IndexDigest(ctx context.Context, repoDir string, result gitbind.Result) error {
	dbPath := filepath.Join(repoDir, DirName, DBFileName)
	if _, err := os.Stat(dbPath); err != nil {
		return nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrate(ctx, db); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `insert or replace into refs(key, value, last_seen_at) values (?, ?, ?)`, "repo.url", result.RepoURL, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `insert or replace into refs(key, value, last_seen_at) values (?, ?, ?)`, "repo.baseCommit", result.BaseCommit, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `insert or replace into refs(key, value, last_seen_at) values (?, ?, ?)`, "repo.headCommit", result.HeadCommit, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `insert or replace into digests(algorithm, digest, kind, path, last_seen_at) values (?, ?, ?, ?, ?)`, result.Patch.Algorithm, result.Patch.Digest, "patch", "patch.diff", now); err != nil {
		return err
	}
	for _, file := range result.ChangedFiles {
		if file.SHA256 == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, `insert or replace into digests(algorithm, digest, kind, path, last_seen_at) values (?, ?, ?, ?, ?)`, "sha256", file.SHA256, "file", file.Path, now); err != nil {
			return err
		}
	}
	_, err = db.ExecContext(ctx, `insert or replace into timestamps(name, value, last_seen_at) values (?, ?, ?)`, "digest.lastComputedAt", now, now)
	return err
}

func migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`create table if not exists refs (
			key text primary key,
			value text not null,
			last_seen_at text not null
		)`,
		`create table if not exists digests (
			algorithm text not null,
			digest text not null,
			kind text not null,
			path text,
			last_seen_at text not null,
			primary key (algorithm, digest, kind, path)
		)`,
		`create table if not exists timestamps (
			name text primary key,
			value text not null,
			last_seen_at text not null
		)`,
		`pragma user_version = 1`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate cache: %w", err)
		}
	}
	return nil
}
