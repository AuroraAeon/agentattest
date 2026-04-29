package gitbind

import "testing"

func TestNormalizeRepoURL(t *testing.T) {
	tests := map[string]string{
		"https://github.com/example/agentattest.git":              "https://github.com/example/agentattest",
		"https://token:secret@github.com/example/agentattest.git": "https://github.com/example/agentattest",
		"git@github.com:example/agentattest.git":                  "https://github.com/example/agentattest",
	}
	for input, want := range tests {
		got, err := NormalizeRepoURL(input)
		if err != nil {
			t.Fatalf("NormalizeRepoURL(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeRepoURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeRepoURLRejectsUnsupportedSchemes(t *testing.T) {
	if _, err := NormalizeRepoURL("ssh://github.com/example/agentattest.git"); err == nil {
		t.Fatal("expected ssh:// remote to be rejected")
	}
}
