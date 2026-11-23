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

func TestCollectSelectedPatterns(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".gitignore"), "skip.go\n")
	mustWrite(t, filepath.Join(root, "root.go"), "root")
	mustWrite(t, filepath.Join(root, "skip.go"), "skip")
	mustWrite(t, filepath.Join(root, "dir", "nested.go"), "nested")
	mustWrite(t, filepath.Join(root, "dir", "note.md"), "note")

	files, err := collectSelected(root, 0, []string{"**/*.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "dir", "nested.go"),
		filepath.Join(root, "root.go"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("want %v, got %v", want, files)
	}
}

func TestCollectSelectedPatternsRespectLevel(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "root.go"), "root")
	mustWrite(t, filepath.Join(root, "dir", "nested.go"), "nested")

	files, err := collectSelected(root, 1, []string{"**/*.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "root.go")}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("want %v, got %v", want, files)
	}
}

func TestCollectSelectedIgnoresUnmatchedPattern(t *testing.T) {
	files, err := collectSelected(t.TempDir(), 0, []string{"*.go"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("want no files, got %v", files)
	}
}

func TestParseToml(t *testing.T) {
	cfg := parseToml("level = 2\nfiles = [\"a.txt\", \"b/c.go\"]\nexcludes = [\".git\", \"node_modules\"]")
	if cfg.level != 2 {
		t.Fatalf("want level=2, got %d", cfg.level)
	}
	want := []string{"a.txt", "b/c.go"}
	if !reflect.DeepEqual(cfg.files, want) {
		t.Fatalf("want %v, got %v", want, cfg.files)
	}
	want = []string{".git", "node_modules"}
	if !reflect.DeepEqual(cfg.excludes, want) {
		t.Fatalf("want %v, got %v", want, cfg.excludes)
	}
}

func TestReadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	mustWrite(t, path, "excludes = [\"node_modules\"]")
	cfg, err := readConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.excludes, []string{"node_modules"}) {
		t.Fatalf("unexpected excludes: %v", cfg.excludes)
	}
}

func TestReadMissingConfigFile(t *testing.T) {
	cfg, err := readConfigFile(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, config{}) {
		t.Fatalf("want empty config, got %#v", cfg)
	}
}

func TestPickExcludes(t *testing.T) {
	got := pickExcludes([]string{"node_modules"}, []string{"tmp"})
	want := []string{"node_modules", "tmp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestParseOptionsFlagsAfterRoot(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "a.txt"), "a")

	opts, err := parseOptions([]string{root, "-f", "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.root != root {
		t.Fatalf("want root=%s, got %s", root, opts.root)
	}
	want := []string{"a.txt"}
	if !reflect.DeepEqual(opts.files, want) {
		t.Fatalf("want files=%v, got %v", want, opts.files)
	}
}

func TestParseOptionsExcludesAfterRoot(t *testing.T) {
	root := t.TempDir()
	opts, err := parseOptions([]string{root, "-e", "tmp", "-e=*.log"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"tmp", "*.log"}
	if !reflect.DeepEqual(opts.excludes, want) {
		t.Fatalf("want excludes=%v, got %v", want, opts.excludes)
	}
}

func TestParseOptionsVersion(t *testing.T) {
	opts, err := parseOptions([]string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.version {
		t.Fatal("want version=true")
	}
}

func TestParseOptionsLevelAfterRoot(t *testing.T) {
	root := t.TempDir()
	opts, err := parseOptions([]string{root, "-l", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.level != 2 {
		t.Fatalf("want level=2, got %d", opts.level)
	}
	if opts.root != root {
		t.Fatalf("want root=%s, got %s", root, opts.root)
	}
}

func TestParseOptionsPositionalFiles(t *testing.T) {
	opts, err := parseOptions([]string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.root != "." {
		t.Fatalf("want root=., got %s", opts.root)
	}
	want := []string{"a.txt", "b.txt"}
	if !reflect.DeepEqual(opts.files, want) {
		t.Fatalf("want files=%v, got %v", want, opts.files)
	}
}

func TestParseOptionsSingleFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	mustWrite(t, path, "a")

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatal(err)
		}
	})

	opts, err := parseOptions([]string{"a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.root != "." {
		t.Fatalf("want root=., got %s", opts.root)
	}
	want := []string{"a.txt"}
	if !reflect.DeepEqual(opts.files, want) {
		t.Fatalf("want files=%v, got %v", want, opts.files)
	}
}

func TestResolveArgsSinglePattern(t *testing.T) {
	root, files := resolveArgs([]string{"*.go"}, nil, []string{"config.txt"})
	if root != "." {
		t.Fatalf("want root=., got %s", root)
	}
	want := []string{"*.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("want files=%v, got %v", want, files)
	}
}

func TestResolveArgsPositionalFilesOverrideConfig(t *testing.T) {
	root, files := resolveArgs(
		[]string{"a.txt", "b.txt"},
		nil,
		[]string{"config.txt"},
	)
	if root != "." {
		t.Fatalf("want root=., got %s", root)
	}
	want := []string{"a.txt", "b.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("want files=%v, got %v", want, files)
	}
}

func TestCollectTargetsExcludesFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "keep.txt"), "keep")
	mustWrite(t, filepath.Join(root, "secret.txt"), "secret")
	mustWrite(t, filepath.Join(root, "tmp", "cache.txt"), "cache")
	mustWrite(t, filepath.Join(root, "nested", "tmp", "cache.txt"), "cache")

	files, err := collectTargets(options{
		root: root, excludes: []string{"secret.txt", "tmp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "keep.txt")}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("want %v, got %v", want, files)
	}
}

func TestCollectTargetsExcludesExplicitFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "tmp", "cache.txt"), "cache")

	files, err := collectTargets(options{
		root: root, files: []string{"tmp/cache.txt"}, excludes: []string{"tmp/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("want no files, got %v", files)
	}
}

func TestMatchesExcludePatterns(t *testing.T) {
	cases := []struct {
		path, pattern string
	}{
		{"dir/generated.go", "**/*.go"},
		{"dir/tmp/file.txt", "tmp"},
		{"build/output.txt", "build/"},
	}
	for _, tc := range cases {
		if !matchesExclude(tc.path, []string{tc.pattern}) {
			t.Errorf("%q should match %q", tc.path, tc.pattern)
		}
	}
}

func TestVersionString(t *testing.T) {
	if got := versionString(); got != "catr 0.2.0" {
		t.Fatalf("want catr 0.2.0, got %s", got)
	}
}

func TestDetectLangVB(t *testing.T) {
	got := detectLang("sample.vb")
	if got != "vbnet" {
		t.Fatalf("want vbnet, got %s", got)
	}
}

func TestDetectLangSQL(t *testing.T) {
	got := detectLang("schema.sql")
	if got != "sql" {
		t.Fatalf("want sql, got %s", got)
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
