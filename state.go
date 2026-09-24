package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type appState struct {
	ProjectUID string `json:"project_uid,omitempty"`
	Filter     string `json:"filter"`
}

func statePath() (string, error) {
	dir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func loadState() (appState, error) {
	state := appState{Filter: "today"}
	path, err := statePath()
	if err != nil {
		return state, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return appState{Filter: "today"}, err
	}
	if state.Filter != "all" && state.Filter != "today" && state.Filter != "upcoming" && state.Filter != "unplanned" {
		state.Filter = "today"
	}
	return state, nil
}

func saveState(state appState) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
