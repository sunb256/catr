# catr

`catr` is a small Go CLI that prints source files as Markdown code blocks.

## Features

- Dumps files under a target directory
- Respects `.gitignore` by default
- Supports depth limit with `-l`
- Supports explicit file and wildcard selection with `-f`
- Supports file and directory exclusion with `-e`
- Optionally reads defaults from `~/.config/catr/config.toml`


## Build

```bash
go build -o catr ./cmd/catr
```

## Install (make `catr` available in PATH)

macOS/Linux (user-local):

```bash
mkdir -p ~/bin
cp ./catr ~/bin/catr
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

Verify:

```bash
which catr
catr -f go.mod .
```


## Usage

```bash
catr [path] [-l depth] [-f file ...] [-e pattern ...]
catr file1 file2 ...
catr --version
```

- `path`: target directory (default: `.`)
- `-l`: max depth (`0` means unlimited)
- `-f`: select files or wildcard patterns (can be repeated)
- `-e`: exclude files or directories by name or pattern (can be repeated)
- `file1 file2 ...`: print only those files or patterns under the current directory


## Examples

```bash
# Basic:
catr .
catr . -f cmd/catr/main.go

# Depth limit:
catr . -l 2

# Specific files:
catr go.mod cmd/catr/main.go

# Specific files with `-f`:
catr -f go.mod -f cmd/catr/main.go .

# Wildcard
catr . -f '**/*.go'
catr '*.md'

# Exclude files or directories:
catr . -e node_modules -e '*.log'
catr . -e 'tmp/**'
```


## Config (`~/.config/catr/config.toml`)

`catr` reads this optional common configuration file when it exists. If it does
not exist, `catr` runs without configuration defaults.

Example:

```toml
level = 2
files = ["go.mod", "cmd/catr/**/*.go"]
excludes = [
  ".git",
  "node_modules",
  ".DS_Store",
  "dist",
  "build",
  "coverage",
]
```


## Output Format

Each file is printed as:

```text
./relative/path
```lang
<file content>
```
```

Language labels are inferred from extension (for example: `go`, `typescript`, `python`, `yaml`, `vbnet`, `sql`).
