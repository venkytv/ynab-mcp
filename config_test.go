package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	// Create a temporary home directory with a config file
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

	// Override HOME so loadConfigFile finds our temp config
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Clear any existing values
	for _, key := range []string{"YNAB_API_TOKEN", "YNAB_BUDGET_ID", "EMPTY_LINE_ABOVE", "SPACES_AROUND", "SINGLE_QUOTED"} {
		t.Setenv(key, "")
	}

	loadConfigFile()

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

func TestLoadConfigFile_EnvVarTakesPrecedence(t *testing.T) {
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".config", "ynab-mcp")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config"), []byte("YNAB_API_TOKEN=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	t.Setenv("YNAB_API_TOKEN", "from-env")

	loadConfigFile()

	if got := os.Getenv("YNAB_API_TOKEN"); got != "from-env" {
		t.Errorf("YNAB_API_TOKEN = %q, want %q (env should take precedence)", got, "from-env")
	}
}

func TestLoadConfigFile_MissingFile(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Should not panic or error when file doesn't exist
	loadConfigFile()
}
