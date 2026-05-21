# renamer

A CLI tool to rename files and directories using regular expressions.

## Installation

```bash
go install github.com/shimapaca/renamer@latest
```

Or build from source:

```bash
git clone https://github.com/shimapaca/renamer
cd renamer
go build -o renamer .
```

## Usage

### Interactive mode

Launch without arguments to enter the interactive TUI. Pattern and replacement are applied in real time as you type.

```bash
renamer [-r] [directory]
```

| Key | Action |
| --- | ------ |
| Type | Edit pattern / replacement, preview updates instantly |
| `Tab` | Switch between Pattern and Replacement fields |
| `Enter` | Execute (only when there are matches and no conflicts) |
| `Ctrl+C` / `Esc` | Quit |

### CLI mode

```bash
renamer [-r] [-n] <pattern> <replacement> [directory]
```

| Argument | Description |
| -------- | ----------- |
| `pattern` | RE2 regular expression matched against each filename (basename only) |
| `replacement` | Replacement string; capture groups referenced as `$1`, `$2`, ... |
| `directory` | Target directory (default: current directory) |

| Flag | Description |
| ---- | ----------- |
| `-r` | Recursively process subdirectories |
| `-n` | Dry run: show preview without prompting |

## How it works

1. Finds files and directories whose names match `pattern`
2. Shows a preview of all renames
3. Aborts if any conflict is detected (see below)
4. Prompts `[y/N]` — type `y` to execute, anything else to abort

```text
Preview:
  ./001_foo.txt → ./foo_001.txt
  ./002_bar.txt → ./bar_002.txt
Proceed? [y/N]:
```

## Conflict detection

Renames are validated before execution. The following are treated as conflicts and will abort the operation:

- The destination file already exists (and is not itself being renamed away in the same batch)
- Two or more files would be renamed to the same destination

```text
Preview:
  ./a.txt → ./existing.txt  [ERROR: destination already exists]
1 conflict(s) found. Aborted.
```

## Examples

### Swap number prefix and filename

```bash
renamer '(\d+)_(.+)\.txt' '$2_$1.txt' ./files/
```

```text
Preview:
  files/001_foo.txt → files/foo_001.txt
  files/002_bar.txt → files/bar_002.txt
Proceed? [y/N]: y
Renamed: files/001_foo.txt → files/foo_001.txt
Renamed: files/002_bar.txt → files/bar_002.txt
```

### Bulk rename extensions (dry run)

```bash
renamer -n '\.jpeg$' '.jpg' ./images/
```

### Remove prefix recursively

```bash
renamer -r '^tmp_' '' ./data/
```

### Replace spaces with underscores

```bash
renamer ' ' '_'
```

## Capture groups

Reference capture groups with `$1`, `$2`, .... Adjacency to `_` and other characters is handled correctly (internally converted to `${1}` form).

```bash
renamer '([a-z]+)_([0-9]+)' '$2_$1'
```

## Notes

- Pattern is matched against the filename (basename) only, not the full path
- In recursive mode (`-r`), deeper paths are renamed first so that renaming a directory does not invalidate its children's paths

## License

[MIT](LICENSE)
