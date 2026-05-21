package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type renameOp struct {
	oldPath string
	newPath string
	depth   int
}

type conflictError struct {
	op     renameOp
	reason string
}

func (e conflictError) Error() string {
	return fmt.Sprintf("%s → %s: %s", e.op.oldPath, e.op.newPath, e.reason)
}

// normalizeReplacement converts $N to ${N} so that $1_foo isn't parsed as group "1_foo".
func normalizeReplacement(replacement string) string {
	re := regexp.MustCompile(`\$\{\d+\}|\$(\d+)`)
	return re.ReplaceAllStringFunc(replacement, func(s string) string {
		if strings.HasPrefix(s, "${") {
			return s
		}
		return "${" + s[1:] + "}"
	})
}

func collectRenames(dir string, re *regexp.Regexp, replacement string, recursive bool) ([]renameOp, error) {
	var ops []renameOp

	addOp := func(path, name string, depth int) {
		newName := re.ReplaceAllString(name, replacement)
		if newName == name {
			return
		}
		ops = append(ops, renameOp{
			oldPath: path,
			newPath: filepath.Join(filepath.Dir(path), newName),
			depth:   depth,
		})
	}

	if recursive {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path == dir {
				return nil
			}
			addOp(path, info.Name(), strings.Count(path, string(os.PathSeparator)))
			return nil
		})
		if err != nil {
			return nil, err
		}
		// Process deepest paths first so renaming a directory doesn't invalidate child paths.
		sort.Slice(ops, func(i, j int) bool {
			return ops[i].depth > ops[j].depth
		})
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			addOp(filepath.Join(dir, entry.Name()), entry.Name(), 0)
		}
	}

	return ops, nil
}

func validateRenames(ops []renameOp) []conflictError {
	oldPaths := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		oldPaths[op.oldPath] = struct{}{}
	}

	seen := make(map[string]string) // newPath → oldPath
	var errs []conflictError
	for _, op := range ops {
		if prev, dup := seen[op.newPath]; dup {
			errs = append(errs, conflictError{op, fmt.Sprintf("宛先が %s と重複", prev)})
			continue
		}
		seen[op.newPath] = op.oldPath

		if _, isSrcInBatch := oldPaths[op.newPath]; !isSrcInBatch {
			if _, err := os.Lstat(op.newPath); err == nil {
				errs = append(errs, conflictError{op, "宛先ファイルが既に存在します"})
			}
		}
	}
	return errs
}

func executeRenames(ops []renameOp) []error {
	var errs []error
	for _, op := range ops {
		if err := os.Rename(op.oldPath, op.newPath); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
