package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile reads key=value pairs from ~/.config/ynab-mcp/config and sets them
// as environment variables. Existing environment variables take precedence.
func LoadFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	f, err := os.Open(filepath.Join(home, ".config", "ynab-mcp", "config"))
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
