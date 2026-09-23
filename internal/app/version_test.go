package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionDefaultsToDev(t *testing.T) {
	info := Version()
	if info.Version != "dev" || info.Commit != "none" || info.Date != "unknown" {
		t.Fatalf("unexpected default build info: %+v", info)
	}
}

func TestRunVersionEmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := runVersion(&buf); err != nil {
		t.Fatalf("runVersion: %v", err)
	}
	var got VersionInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("version output is not valid JSON (%q): %v", buf.String(), err)
	}
	if got != Version() {
		t.Fatalf("version output %+v does not match Version() %+v", got, Version())
	}
	for _, field := range []string{`"version"`, `"commit"`, `"date"`} {
		if !strings.Contains(buf.String(), field) {
			t.Fatalf("version output missing %s: %q", field, buf.String())
		}
	}
}
