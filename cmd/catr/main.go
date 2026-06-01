package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type config struct {
	level int
	files []string
}

type options struct {
	root  string
	level int
	files []string
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		exitErr(err)
	}
	paths, err := collectTargets(opts)
	if err != nil {
		exitErr(err)
	}
	if err := printFiles(paths); err != nil {
		exitErr(err)
	}
}

func parseOptions(args []string) (options, error) {
	cfg, _ := readConfig()
	args = reorderArgs(args)
	fs := flag.NewFlagSet("catr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	level := fs.Int("l", cfg.level, "max directory depth")
	var files listFlag
	if len(cfg.files) > 0 {
		files = append(files, cfg.files...)
	}
	fs.Var(&files, "f", "target files")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	if *level < 0 {
		return options{}, errors.New("-l must be >= 0")
	}
	return options{root: root, level: *level, files: files}, nil
}

func reorderArgs(args []string) []string {
	opts := []string{}
	pos := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-f" || arg == "-l" {
			opts = append(opts, arg)
			if i+1 < len(args) {
				i++
				opts = append(opts, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "-f=") || strings.HasPrefix(arg, "-l=") {
			opts = append(opts, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") {
			opts = append(opts, arg)
			continue
		}
		pos = append(pos, arg)
	}
	return append(opts, pos...)
}

func collectTargets(opts options) ([]string, error) {
	absRoot, err := filepath.Abs(opts.root)
	if err != nil {
		return nil, err
	}
	if len(opts.files) > 0 {
		return collectFromFiles(absRoot, opts.files)
	}
	ign, err := loadIgnore(absRoot)
	if err != nil {
		return nil, err
	}
	return walkFiles(absRoot, opts.level, ign)
}

func collectFromFiles(root string, files []string) ([]string, error) {
	out := make([]string, 0, len(files))
	seen := map[string]bool{}
	for _, name := range files {
		full := name
		if !filepath.IsAbs(full) {
			full = filepath.Join(root, name)
		}
		info, err := os.Stat(full)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, fmt.Errorf("directory is not supported in -f: %s", name)
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, full)
	}
	sort.Strings(out)
	return out, nil
}

func walkFiles(root string, maxDepth int, ign []string) ([]string, error) {
	out := []string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if shouldSkip(rel, d.IsDir(), ign) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if maxDepth > 0 && depth(rel) > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func printFiles(paths []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	for i, path := range paths {
		if i > 0 {
			fmt.Println()
		}
		rel, err := filepath.Rel(wd, path)
		if err != nil {
			return err
		}
		fmt.Printf("./%s\n", filepath.ToSlash(rel))
		lang := detectLang(path)
		fmt.Printf("```%s\n", lang)
		if err := writeContent(path); err != nil {
			return err
		}
		fmt.Println("```")
	}
	return nil
}

func writeContent(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Print(string(b))
	if len(b) == 0 || b[len(b)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

func loadIgnore(root string) ([]string, error) {
	ign := []string{".git/", ".git"}
	path := filepath.Join(root, ".gitignore")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ign, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ign = append(ign, normalizePattern(line))
	}
	return ign, s.Err()
}

func shouldSkip(rel string, isDir bool, patterns []string) bool {
	for _, p := range patterns {
		if matches(rel, isDir, p) {
			return true
		}
	}
	return false
}

func matches(rel string, isDir bool, pat string) bool {
	if strings.HasSuffix(pat, "/") {
		base := strings.TrimSuffix(pat, "/")
		return rel == base || strings.HasPrefix(rel, base+"/")
	}
	if strings.Contains(pat, "/") {
		ok, _ := filepath.Match(filepath.FromSlash(pat), filepath.FromSlash(rel))
		if ok {
			return true
		}
	}
	name := filepath.Base(rel)
	ok, _ := filepath.Match(pat, name)
	if ok {
		return true
	}
	return !isDir && rel == pat
}

func depth(rel string) int {
	if rel == "" || rel == "." {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

func readConfig() (config, error) {
	cfg := config{}
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	path := filepath.Join(home, ".config", "catr.toml")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return parseToml(string(b)), nil
}

func parseToml(s string) config {
	cfg := config{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "level") {
			cfg.level = parseLevel(line)
			continue
		}
		if strings.HasPrefix(line, "files") {
			cfg.files = parseFiles(line)
		}
	}
	return cfg
}

func parseLevel(line string) int {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return 0
	}
	v := strings.TrimSpace(parts[1])
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0
	}
	if n < 0 {
		return 0
	}
	return n
}

func parseFiles(line string) []string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return nil
	}
	v := strings.TrimSpace(parts[1])
	v = strings.TrimPrefix(v, "[")
	v = strings.TrimSuffix(v, "]")
	if strings.TrimSpace(v) == "" {
		return nil
	}
	list := []string{}
	for _, item := range strings.Split(v, ",") {
		s := strings.TrimSpace(item)
		s = strings.Trim(s, "\"")
		if s != "" {
			list = append(list, s)
		}
	}
	return list
}

func normalizePattern(p string) string {
	if strings.HasPrefix(p, "./") {
		p = strings.TrimPrefix(p, "./")
	}
	return filepath.ToSlash(p)
}

func detectLang(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	m := map[string]string{
		".go":   "go",
		".py":   "python",
		".ts":   "typescript",
		".tsx":  "tsx",
		".js":   "javascript",
		".jsx":  "jsx",
		".json": "json",
		".sql":  "sql",
		".yml":  "yaml",
		".yaml": "yaml",
		".md":   "markdown",
		".sh":   "bash",
		".html": "html",
		".css":  "css",
		".toml": "toml",
		".rs":   "rust",
		".java": "java",
		".vb":   "vbnet",
		".c":    "c",
		".h":    "c",
		".cpp":  "cpp",
	}
	if lang, ok := m[ext]; ok {
		return lang
	}
	return "text"
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

type listFlag []string

func (l *listFlag) String() string {
	return strings.Join(*l, ",")
}

func (l *listFlag) Set(v string) error {
	*l = append(*l, v)
	return nil
}
