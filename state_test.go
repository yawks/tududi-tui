package main

import "testing"

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := appState{ProjectUID: "project-uid", Filter: "upcoming"}
	if err := saveState(want); err != nil {
		t.Fatal(err)
	}
	got, err := loadState()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("state = %#v, want %#v", got, want)
	}
}
