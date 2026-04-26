package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadConfigFile reads key=value pairs from ~/.config/ynab-mcp/config
// and sets them as environment variables. Existing env vars take precedence.
func loadConfigFile() {
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
		// Strip surrounding quotes
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
