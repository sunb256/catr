# catr

`catr` is a small Go CLI that prints source files as Markdown code blocks.

## Features

- Dumps files under a target directory
- Respects `.gitignore` by default
- Supports depth limit with `-l`
- Supports explicit file selection with `-f`
- Optionally reads defaults from `~/.config/catr.toml`
- Works as a single binary

## Build

```bash
go build -o catr ./cmd/catr
```

## Usage

```bash
./catr [path] [-l depth] [-f file ...]
```

- `path`: target directory (default: `.`)
- `-l`: max depth (`0` means unlimited)
- `-f`: select specific files (can be repeated)

## Examples

Basic:

```bash
./catr .
```

Depth limit:

```bash
./catr . -l 2
```

Specific files:

```bash
./catr -f go.mod -f cmd/catr/main.go .
```

`-f` after path is also supported:

```bash
./catr . -f cmd/catr/main.go
```

## Config (`~/.config/catr.toml`)

Example:

```toml
level = 2
files = ["go.mod", "cmd/catr/main.go"]
```

Notes:

- CLI flags override config values.
- If `-f` is set (CLI or config), only those files are printed.

## Output Format

Each file is printed as:

```text
./relative/path
```lang
<file content>
```
```

Language labels are inferred from extension (for example: `go`, `typescript`, `python`, `yaml`, `vbnet`, `sql`).

## Development

Run checks:

```bash
gofmt -w cmd/catr/main.go cmd/catr/main_test.go
go vet ./...
go test ./...
```
