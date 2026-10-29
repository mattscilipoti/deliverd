package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store keeps named groupings of item sets as one JSON file each in Dir.
type Store struct{ Dir string }

const storeExt = ".json"

// DefaultStoreDir is <user config dir>/deliverd/loads.
func DefaultStoreDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "deliverd", "loads"), nil
}

type savedLoad struct {
	Name string    `json:"name"`
	Sets []ItemSet `json:"sets"`
}

// ValidateName returns the trimmed name, or an error if it can't be a file name.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", errors.New("name can't be empty")
	case len(name) > 60:
		return "", errors.New("name must be 60 characters or fewer")
	case strings.HasPrefix(name, "."):
		return "", errors.New("name can't start with a dot")
	case strings.ContainsAny(name, `/\:*?"<>|`) || strings.ContainsFunc(name, func(r rune) bool { return r < 32 }):
		return "", errors.New(`name can't contain / \ : * ? " < > |`)
	}
	return name, nil
}

func (s Store) path(name string) string { return filepath.Join(s.Dir, name+storeExt) }

// Exists reports whether a load with this name is saved.
func (s Store) Exists(name string) bool {
	_, err := os.Stat(s.path(name))
	return err == nil
}

// Save writes sets under name, replacing any existing load of that name. The
// write goes to a temp file first so a failure can't corrupt an existing save.
func (s Store) Save(name string, sets []ItemSet) error {
	name, err := ValidateName(name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(savedLoad{Name: name, Sets: sets}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path(name))
}

func (s Store) Load(name string) ([]ItemSet, error) {
	name, err := ValidateName(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		return nil, err
	}
	var sl savedLoad
	if err := json.Unmarshal(data, &sl); err != nil {
		return nil, fmt.Errorf("%s is not a valid saved load: %w", name, err)
	}
	for i, it := range sl.Sets {
		if it.Qty <= 0 || it.L <= 0 || it.W <= 0 || it.H <= 0 {
			return nil, fmt.Errorf("%s: set #%d has a non-positive quantity or dimension", name, i+1)
		}
		if it.Weight < 0 || math.IsNaN(it.Weight) {
			return nil, fmt.Errorf("%s: set #%d has an invalid weight", name, i+1)
		}
	}
	return sl.Sets, nil
}

// List returns saved load names, sorted. A missing directory means none saved.
func (s Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || strings.HasPrefix(n, ".") || !strings.HasSuffix(n, storeExt) {
			continue
		}
		names = append(names, strings.TrimSuffix(n, storeExt))
	}
	sort.Strings(names)
	return names, nil
}
