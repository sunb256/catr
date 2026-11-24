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

const version = "0.2.0"

type config struct {
	level    int
	files    []string
	excludes []string
}

type options struct {
	root     string
	level    int
	files    []string
	excludes []string
	version  bool
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		exitErr(err)
	}
	if opts.version {
		fmt.Println(versionString())
		return
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
	var cliFiles listFlag
	fs.Var(&cliFiles, "f", "target files")
	var cliExcludes listFlag
	fs.Var(&cliExcludes, "e", "exclude files or directories")
	showVersion := fs.Bool("version", false, "print version")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if *level < 0 {
		return options{}, errors.New("-l must be >= 0")
	}
	root, files := resolveArgs(fs.Args(), cliFiles, cfg.files)
	return options{
		root: root, level: *level, files: files,
		excludes: pickExcludes(cfg.excludes, cliExcludes), version: *showVersion,
	}, nil
}

func versionString() string {
	return "catr " + version
}

func resolveArgs(pos []string, cliFiles []string, cfgFiles []string) (string, []string) {
	root := "."
	if len(pos) == 0 {
		return root, pickFiles(cliFiles, cfgFiles)
	}
	if len(pos) > 1 {
		files := append([]string{}, cliFiles...)
		return root, append(files, pos...)
	}
	if isPattern(pos[0]) {
		files := append([]string{}, cliFiles...)
		return root, append(files, pos[0])
	}
	if len(cliFiles) > 0 {
		return pos[0], cliFiles
	}
	if isFile(pos[0]) {
		return root, []string{pos[0]}
	}
	return pos[0], cfgFiles
}

func pickFiles(cliFiles []string, cfgFiles []string) []string {
	if len(cliFiles) > 0 {
		return cliFiles
	}
	return cfgFiles
}

func pickExcludes(cfgExcludes, cliExcludes []string) []string {
	excludes := append([]string{}, cfgExcludes...)
	return append(excludes, cliExcludes...)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func reorderArgs(args []string) []string {
	opts := []string{}
	pos := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-e" || arg == "-f" || arg == "-l" {
			opts = append(opts, arg)
			if i+1 < len(args) {
				i++
				opts = append(opts, args[i])
			}
			continue
		}
		if strings.HasPrefix(arg, "-e=") || strings.HasPrefix(arg, "-f=") || strings.HasPrefix(arg, "-l=") {
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
		return collectSelected(absRoot, opts.level, opts.files, opts.excludes)
	}
	ign, err := loadIgnore(absRoot)
	if err != nil {
		return nil, err
	}
	return walkFilesExcept(absRoot, opts.level, ign, opts.excludes)
}

func collectSelected(root string, level int, names, excludes []string) ([]string, error) {
	files, patterns := splitTargets(names)
	out, err := collectFromFiles(root, files)
	if err != nil || len(patterns) == 0 {
		return filterExcluded(root, out, excludes), err
	}
	ign, err := loadIgnore(root)
	if err != nil {
		return nil, err
	}
	candidates, err := walkFilesExcept(root, level, ign, excludes)
	if err != nil {
		return nil, err
	}
	out = filterExcluded(root, out, excludes)
	return mergePaths(out, filterPatterns(root, candidates, patterns)), nil
}

func filterExcluded(root string, paths, excludes []string) []string {
	out := []string{}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil || matchesExclude(filepath.ToSlash(rel), excludes) {
			continue
		}
		out = append(out, path)
	}
	return out
}

func splitTargets(names []string) ([]string, []string) {
	files := []string{}
	patterns := []string{}
	for _, name := range names {
		if isPattern(name) {
			patterns = append(patterns, normalizePattern(name))
			continue
		}
		files = append(files, name)
	}
	return files, patterns
}

func isPattern(name string) bool {
	return strings.Contains(name, "*")
}

func filterPatterns(root string, paths []string, patterns []string) []string {
	out := []string{}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if matchesAny(filepath.ToSlash(rel), patterns) {
			out = append(out, path)
		}
	}
	return out
}

func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPath(strings.Split(pattern, "/"), strings.Split(path, "/")) {
			return true
		}
	}
	return false
}

func matchPath(pattern []string, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(path); i++ {
			if matchPath(pattern[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	ok, err := filepath.Match(pattern[0], path[0])
	return err == nil && ok && matchPath(pattern[1:], path[1:])
}

func mergePaths(groups ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, paths := range groups {
		for _, path := range paths {
			if !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	sort.Strings(out)
	return out
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
	return walkFilesExcept(root, maxDepth, ign, nil)
}

func walkFilesExcept(root string, maxDepth int, ign, excludes []string) ([]string, error) {
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
		if shouldSkip(rel, d.IsDir(), ign) || matchesExclude(rel, excludes) {
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

func matchesExclude(rel string, patterns []string) bool {
	for _, raw := range patterns {
		pattern := strings.TrimSuffix(normalizePattern(raw), "/")
		if pattern == "" {
			continue
		}
		if excludeMatch(rel, pattern) {
			return true
		}
	}
	return false
}

func excludeMatch(rel, pattern string) bool {
	path := strings.Split(rel, "/")
	if !strings.Contains(pattern, "/") {
		return matchName(path, pattern)
	}
	parts := strings.Split(pattern, "/")
	for i := 1; i <= len(path); i++ {
		if matchPath(parts, path[:i]) {
			return true
		}
	}
	return false
}

func matchName(path []string, pattern string) bool {
	for _, name := range path {
		ok, err := filepath.Match(pattern, name)
		if err == nil && ok {
			return true
		}
	}
	return false
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
	path := filepath.Join(home, ".config", "catr", "config.toml")
	return readConfigFile(path)
}

func readConfigFile(path string) (config, error) {
	cfg := config{}
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
			continue
		}
		if strings.HasPrefix(line, "excludes") {
			cfg.excludes = parseFiles(line)
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
