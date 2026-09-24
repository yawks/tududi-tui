package main

import "time"

type task struct {
	ID         int      `json:"id"`
	UID        string   `json:"uid"`
	Name       string   `json:"name"`
	Note       string   `json:"note"`
	Status     any      `json:"status"`
	Priority   any      `json:"priority"`
	DueDate    string   `json:"due_date"`
	CreatedAt  string   `json:"created_at"`
	ProjectID  int      `json:"project_id"`
	ProjectUID string   `json:"project_uid"`
	Project    *project `json:"Project"`
	Tags       []tag    `json:"tags"`
}

func priorityName(v any) string {
	switch v := v.(type) {
	case string:
		if v != "" {
			return v
		}
	case float64:
		if v >= 0 && v < 3 {
			return []string{"low", "medium", "high"}[int(v)]
		}
	case int:
		if v >= 0 && v < 3 {
			return []string{"low", "medium", "high"}[v]
		}
	}
	return "medium"
}

func (t task) completed() bool {
	switch v := t.Status.(type) {
	case string:
		return v == "done" || v == "completed"
	case float64:
		return v == 2
	default:
		return false
	}
}

func (t task) inProgress() bool {
	switch v := t.Status.(type) {
	case string:
		return v == "in_progress"
	case float64:
		return v == 1
	case int:
		return v == 1
	default:
		return false
	}
}

func (t task) due() (time.Time, bool) {
	if t.DueDate == "" {
		return time.Time{}, false
	}
	d, err := time.Parse(time.RFC3339, t.DueDate)
	if err != nil {
		d, err = time.Parse("2006-01-02", t.DueDate)
	}
	return d, err == nil
}

func (t task) created() (time.Time, bool) {
	d, err := time.Parse(time.RFC3339, t.CreatedAt)
	return d, err == nil
}

type project struct {
	ID          int    `json:"id"`
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    any    `json:"priority"`
	Color       string `json:"color"`
}

type tag struct {
	ID    int    `json:"id"`
	UID   string `json:"uid"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
