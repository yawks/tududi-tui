package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientUsesBearerAndTagColor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tag/abc" || r.Method != http.MethodPatch {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tt_test" {
			t.Fatalf("authorization = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["color"] != "#ff0000" {
			t.Fatalf("color = %v", body["color"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newClient(config{BaseURL: server.URL, APIKey: "tt_test"})
	if err := c.saveTag(tag{UID: "abc", Name: "urgent", Color: "#ff0000"}, false); err != nil {
		t.Fatal(err)
	}
}

func TestCreateTagAddsColorWithPatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/api/tag" {
				t.Fatalf("unexpected create request %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uid":"new-tag","name":"urgent"}`))
		case 2:
			if r.Method != http.MethodPatch || r.URL.Path != "/api/tag/new-tag" {
				t.Fatalf("unexpected color request %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	c := newClient(config{BaseURL: server.URL, APIKey: "tt_test"})
	if err := c.saveTag(tag{Name: "urgent", Color: "#ff0000"}, true); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestToggleTaskUsesTaskPatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/task/task-uid" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["status"] != "done" {
			t.Fatalf("status = %q, want done", body["status"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := newClient(config{BaseURL: server.URL, APIKey: "tt_test"})
	if err := c.toggleTask(task{UID: "task-uid", Status: float64(0)}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRequestsAllTaskStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tasks":
			if r.URL.Query().Get("type") != "all" || r.URL.Query().Get("status") != "all" {
				t.Fatalf("task query = %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"tasks":[{"name":"done","status":2}]}`))
		case "/api/projects":
			_, _ = w.Write([]byte(`[]`))
		case "/api/tags":
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected request %s", r.URL.String())
		}
	}))
	defer server.Close()

	tasks, _, _, err := newClient(config{BaseURL: server.URL}).load()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].completed() {
		t.Fatalf("completed tasks = %#v", tasks)
	}
}
