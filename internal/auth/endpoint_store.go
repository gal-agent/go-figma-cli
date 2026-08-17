package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// SaveEndpoint persists the discovered endpoint next to the token so later
// refreshes do not need to redo discovery/registration.
func (s *Store) SaveEndpoint(ep *Endpoint) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path+".endpoint.json", raw, 0o600)
}

// LoadEndpoint returns the persisted endpoint, or nil.
func (s *Store) LoadEndpoint() (*Endpoint, error) {
	raw, err := os.ReadFile(s.Path + ".endpoint.json")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ep Endpoint
	if err := json.Unmarshal(raw, &ep); err != nil {
		return nil, nil // corrupt -> force re-login
	}
	return &ep, nil
}
