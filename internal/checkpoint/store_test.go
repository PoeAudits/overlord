package checkpoint

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveArchivesPreviousCurrentAndKeepsTwo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace-checkpoints.yaml")
	for _, name := range []string{"one", "two", "three", "four"} {
		if err := Save(path, &Store{Current: &Checkpoint{SavedAt: time.Date(2026, 1, len(name), 0, 0, 0, 0, time.UTC), Sessions: []Session{{Name: name}}}}); err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Current.Sessions[0].Name != "four" || len(got.Archives) != 2 || got.Archives[0].Sessions[0].Name != "three" || got.Archives[1].Sessions[0].Name != "two" {
		t.Fatalf("unexpected retention: %#v", got)
	}
}

func TestRevertPromotesArchiveWithoutDiscardingCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspace-checkpoints.yaml")
	store := &Store{Version: 1, Current: &Checkpoint{Sessions: []Session{{Name: "current"}}}, Archives: []Checkpoint{{Sessions: []Session{{Name: "newer"}}}, {Sessions: []Session{{Name: "older"}}}}}
	if err := write(path, store); err != nil {
		t.Fatal(err)
	}
	if err := Revert(path, 1); err != nil {
		t.Fatal(err)
	}
	got, _ := Load(path)
	if got.Current.Sessions[0].Name != "older" || got.Archives[0].Sessions[0].Name != "current" || got.Archives[1].Sessions[0].Name != "newer" {
		t.Fatalf("unexpected revert: %#v", got)
	}
}
