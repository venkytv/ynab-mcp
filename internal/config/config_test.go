package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".config", "ynab-mcp")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}

	envContent := `# Comment line
YNAB_API_TOKEN=from-config-file
YNAB_BUDGET_ID="quoted-budget-id"
EMPTY_LINE_ABOVE=works

  SPACES_AROUND = trimmed
SINGLE_QUOTED='single'
NO_EQUALS_SIGN
`
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte(envContent), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", tmpHome)
	for _, key := range []string{"YNAB_API_TOKEN", "YNAB_BUDGET_ID", "EMPTY_LINE_ABOVE", "SPACES_AROUND", "SINGLE_QUOTED"} {
		t.Setenv(key, "")
	}

	LoadFile()

	tests := []struct {
		key  string
		want string
	}{
		{"YNAB_API_TOKEN", "from-config-file"},
		{"YNAB_BUDGET_ID", "quoted-budget-id"},
		{"EMPTY_LINE_ABOVE", "works"},
		{"SPACES_AROUND", "trimmed"},
		{"SINGLE_QUOTED", "single"},
	}
	for _, tt := range tests {
		if got := os.Getenv(tt.key); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestLoadFileEnvVarTakesPrecedence(t *testing.T) {
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".config", "ynab-mcp")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte("YNAB_API_TOKEN=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", tmpHome)
	t.Setenv("YNAB_API_TOKEN", "from-env")

	LoadFile()

	if got := os.Getenv("YNAB_API_TOKEN"); got != "from-env" {
		t.Errorf("YNAB_API_TOKEN = %q, want %q", got, "from-env")
	}
}

func TestLoadFileMissingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	LoadFile()
}
