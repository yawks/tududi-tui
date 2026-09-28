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
	Sort       string `json:"sort,omitempty"`
	SortDesc   bool   `json:"sort_desc,omitempty"`
	Priority   string `json:"priority,omitempty"`
	Status     string `json:"status,omitempty"`
	Tags       string `json:"tags,omitempty"`
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
	if state.Filter != "all" && state.Filter != "today" && state.Filter != "upcoming" && state.Filter != "unplanned" && state.Filter != "done" && state.Filter != "dashboard" {
		state.Filter = "today"
	}
	if state.Sort != "" && state.Sort != "date" && state.Sort != "name" && state.Sort != "priority" && state.Sort != "tag" {
		state.Sort = ""
	}
	if state.Priority != "" && state.Priority != "low" && state.Priority != "medium" && state.Priority != "high" {
		state.Priority = ""
	}
	if state.Status != "" && statusName(state.Status) != state.Status {
		state.Status = ""
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
