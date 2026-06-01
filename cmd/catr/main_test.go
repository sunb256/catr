package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWalkFilesRespectsGitignore(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "skip.txt\ndir/\n")
	mustWrite(t, filepath.Join(root, "keep.txt"), "ok")
	mustWrite(t, filepath.Join(root, "skip.txt"), "ng")
	mustWrite(t, filepath.Join(root, "dir", "a.txt"), "ng")

	ign, err := loadIgnore(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := walkFiles(root, 0, ign)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("unexpected files: %#v", files)
	}
}

func TestWalkFilesLevel(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "a")
	mustWrite(t, filepath.Join(root, "x", "b.txt"), "b")
	mustWrite(t, filepath.Join(root, "x", "y", "c.txt"), "c")

	files, err := walkFiles(root, 2, []string{".git/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d", len(files))
	}
}

func TestCollectFromFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "a")
	mustWrite(t, filepath.Join(root, "b.txt"), "b")

	files, err := collectFromFiles(root, []string{"b.txt", "a.txt", "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d", len(files))
	}
}

func TestParseToml(t *testing.T) {
	cfg := parseToml("level = 2\nfiles = [\"a.txt\", \"b/c.go\"]")
	if cfg.level != 2 {
		t.Fatalf("want level=2, got %d", cfg.level)
	}
	want := []string{"a.txt", "b/c.go"}
	if !reflect.DeepEqual(cfg.files, want) {
		t.Fatalf("want %v, got %v", want, cfg.files)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
