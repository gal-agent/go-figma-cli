// Package config stores the Figma personal access token (PAT) on disk.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Store is the on-disk configuration.
type Store struct {
	PAT string `json:"pat"`
}

// DefaultPath returns the config file location:
// $FIGMA_CLI_CONFIG_HOME/config.json when set, otherwise
// <os.UserConfigDir>/figma-cli/config.json.
func DefaultPath() string {
	if env := os.Getenv("FIGMA_CLI_CONFIG_HOME"); env != "" {
		return filepath.Join(env, "config.json")
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "figma-cli", "config.json")
}

// Load reads the config file; returns nil when it does not exist.
func Load() (*Store, error) {
	raw, err := os.ReadFile(DefaultPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Store
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("config %s is corrupt: %w", DefaultPath(), err)
	}
	return &s, nil
}

// Save writes the config file with 0600 permissions.
func Save(s *Store) error {
	p := DefaultPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}

// Clear removes the config file (logout).
func Clear() error {
	err := os.Remove(DefaultPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Token returns the PAT: $FIGMA_TOKEN wins over the config file.
func Token() (string, error) {
	if tok := os.Getenv("FIGMA_TOKEN"); tok != "" {
		return tok, nil
	}
	s, err := Load()
	if err != nil {
		return "", err
	}
	if s == nil || s.PAT == "" {
		return "", errors.New("no Figma personal access token configured; create one at https://www.figma.com/settings (Security -> Personal access tokens) and run `go-figma-cli login --token <PAT>`")
	}
	return s.PAT, nil
}
