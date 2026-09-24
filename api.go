package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type client struct {
	base, token string
	http        *http.Client
}

func newClient(cfg config) *client {
	return &client{base: cfg.BaseURL, token: cfg.APIKey, http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *client) do(method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+"/api/"+strings.TrimLeft(path, "/"), r)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}

func (c *client) load() ([]task, []project, []tag, error) {
	var raw json.RawMessage
	if err := c.do(http.MethodGet, "tasks?type=all&status=all", nil, &raw); err != nil {
		return nil, nil, nil, err
	}
	var tasks []task
	if len(raw) > 0 && raw[0] == '[' {
		if err := json.Unmarshal(raw, &tasks); err != nil {
			return nil, nil, nil, err
		}
	} else {
		var wrapped struct {
			Tasks []task `json:"tasks"`
		}
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return nil, nil, nil, err
		}
		tasks = wrapped.Tasks
	}
	var projectRaw json.RawMessage
	if err := c.do(http.MethodGet, "projects?status=all", nil, &projectRaw); err != nil {
		return nil, nil, nil, err
	}
	var projects []project
	if len(projectRaw) > 0 && projectRaw[0] == '[' {
		if err := json.Unmarshal(projectRaw, &projects); err != nil {
			return nil, nil, nil, err
		}
	} else {
		var wrapped struct {
			Projects []project `json:"projects"`
		}
		if err := json.Unmarshal(projectRaw, &wrapped); err != nil {
			return nil, nil, nil, err
		}
		projects = wrapped.Projects
	}
	var tags []tag
	if err := c.do(http.MethodGet, "tags", nil, &tags); err != nil {
		return nil, nil, nil, err
	}
	return tasks, projects, tags, nil
}

func ident(uid string, id int) string {
	if uid != "" {
		return url.PathEscape(uid)
	}
	return strconv.Itoa(id)
}

func (c *client) saveTask(t task, create bool) error {
	payload := map[string]any{"name": t.Name, "note": t.Note, "priority": priorityName(t.Priority), "status": "not_started"}
	if t.ProjectID != 0 {
		payload["project_id"] = t.ProjectID
	}
	if t.DueDate != "" {
		payload["due_date"] = t.DueDate
	} else if !create {
		payload["due_date"] = nil
	}
	names := make([]map[string]string, len(t.Tags))
	for i, v := range t.Tags {
		names[i] = map[string]string{"name": v.Name}
	}
	payload["tags"] = names
	method, path := http.MethodPost, "task"
	if !create {
		method, path = http.MethodPatch, "task/"+ident(t.UID, t.ID)
	}
	return c.do(method, path, payload, nil)
}

func (c *client) toggleTask(t task) error {
	status := "done"
	if t.completed() {
		status = "not_started"
	}
	return c.do(http.MethodPatch, "task/"+ident(t.UID, t.ID), map[string]string{"status": status}, nil)
}
func (c *client) deleteTask(t task) error {
	return c.do(http.MethodDelete, "task/"+ident(t.UID, t.ID), nil, nil)
}

func (c *client) saveProject(p project, create bool) error {
	payload := map[string]any{"name": p.Name, "description": p.Description, "priority": priorityName(p.Priority), "status": p.Status, "color": p.Color}
	method, path := http.MethodPost, "project"
	if !create {
		method, path = http.MethodPatch, "project/"+ident(p.UID, p.ID)
	}
	return c.do(method, path, payload, nil)
}
func (c *client) deleteProject(p project) error {
	return c.do(http.MethodDelete, "project/"+ident(p.UID, p.ID), nil, nil)
}

func (c *client) saveTag(t tag, create bool) error {
	if create {
		var created tag
		if err := c.do(http.MethodPost, "tag", map[string]any{"name": t.Name}, &created); err != nil {
			return err
		}
		if t.Color == "" {
			return nil
		}
		return c.do(http.MethodPatch, "tag/"+ident(created.UID, created.ID), map[string]any{"color": t.Color}, nil)
	}
	return c.do(http.MethodPatch, "tag/"+ident(t.UID, t.ID), map[string]any{"name": t.Name, "color": t.Color}, nil)
}
func (c *client) deleteTag(t tag) error {
	return c.do(http.MethodDelete, "tag/"+ident(t.UID, t.ID), nil, nil)
}
