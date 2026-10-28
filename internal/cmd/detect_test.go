package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetectScansCategoriesAndArchive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base := t.TempDir()
	for _, path := range []string{"projects/web/known", "projects/web/new", "archive/old"} {
		if err := os.MkdirAll(filepath.Join(base, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "projects/web/file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	targets, err := detectionTargets("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := scanUnregisteredProjects(base, map[string]struct{}{"projects/web/known": {}}, targets)
	if err != nil {
		t.Fatal(err)
	}
	want := []unregisteredProject{
		{Name: "new", Path: "projects/web/new", Category: "web"},
		{Name: "old", Path: "archive/old", Category: "archive"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projects = %#v, want %#v", got, want)
	}
}

func TestDetectCategorySelection(t *testing.T) {
	targets, err := detectionTargets("archive")
	if err != nil || !reflect.DeepEqual(targets, []scanTarget{{dir: "archive", category: "archive"}}) {
		t.Fatalf("archive targets = %#v, error = %v", targets, err)
	}
	if _, err := detectionTargets("unknown"); err == nil {
		t.Fatal("expected an unknown category error")
	}
}
