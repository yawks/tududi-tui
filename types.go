package main

import (
	"strings"
	"time"
)

var taskStatuses = []string{"not_started", "planned", "in_progress", "waiting", "done", "cancelled"}

func statusName(v any) string {
	switch v := v.(type) {
	case string:
		v = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(v), " ", "_"))
		if v == "completed" {
			return "done"
		}
		for _, status := range taskStatuses {
			if v == status {
				return v
			}
		}
	case float64:
		if v >= 0 && v < 3 {
			return []string{"not_started", "in_progress", "done"}[int(v)]
		}
	case int:
		if v >= 0 && v < 3 {
			return []string{"not_started", "in_progress", "done"}[v]
		}
	}
	return "not_started"
}

func statusLabel(v any) string {
	return strings.Title(strings.ReplaceAll(statusName(v), "_", " "))
}

func statusIcon(v any) string {
	return map[string]string{"planned": "📅", "in_progress": "🚧", "waiting": "⏳", "done": "✓", "cancelled": "🚫"}[statusName(v)]
}

func statusColor(v any) string {
	return map[string]string{"not_started": "#8b949e", "planned": "#3b82f6", "in_progress": "#f59e0b", "waiting": "#eab308", "done": "#22c55e", "cancelled": "#ef4444"}[statusName(v)]
}

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
		v = strings.ToLower(strings.TrimSpace(v))
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

func priorityLabel(v any) string { return strings.Title(priorityName(v)) }

func priorityColor(v any) string {
	return map[string]string{"low": "#3b82f6", "medium": "#f59e0b", "high": "#ef4444"}[priorityName(v)]
}

func (t task) completed() bool {
	return statusName(t.Status) == "done"
}

func (t task) inProgress() bool {
	return statusName(t.Status) == "in_progress"
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
