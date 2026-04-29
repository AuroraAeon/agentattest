package cache

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesOnlyCacheTables(t *testing.T) {
	dir := t.TempDir()
	dbPath, err := Init(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, DirName, ConfigName)); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`select name from sqlite_master where type = 'table' order by name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
		if strings.Contains(name, "verification") || strings.Contains(name, "result") {
			t.Fatalf("cache must not store verification result table %q", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := "digests,refs,timestamps"
	if strings.Join(tables, ",") != want {
		t.Fatalf("tables = %v, want %s", tables, want)
	}
}
