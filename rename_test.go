package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// mkfiles creates files and directories under dir according to the paths slice.
// Paths ending with "/" are created as directories.
func mkfiles(t *testing.T, dir string, paths []string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if p[len(p)-1] == '/' {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func mustCompile(t *testing.T, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("invalid pattern %q: %v", pattern, err)
	}
	return re
}

func TestNormalizeReplacement(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"$1", "${1}"},
		{"$2_$1.txt", "${2}_${1}.txt"},
		{"${1}_$2", "${1}_${2}"},
		{"prefix_$1_suffix", "prefix_${1}_suffix"},
		{"no-groups", "no-groups"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeReplacement(tt.input)
			if got != tt.want {
				t.Errorf("normalizeReplacement(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCollectRenames_Flat(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"001_foo.txt", "002_bar.txt", "readme.md"})

	re := mustCompile(t, `(\d+)_(.+)\.txt`)
	replacement := normalizeReplacement("$2_$1.txt")
	ops, err := collectRenames(dir, re, replacement, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(ops) != 2 {
		t.Fatalf("got %d ops, want 2", len(ops))
	}

	want := map[string]string{
		filepath.Join(dir, "001_foo.txt"): filepath.Join(dir, "foo_001.txt"),
		filepath.Join(dir, "002_bar.txt"): filepath.Join(dir, "bar_002.txt"),
	}
	for _, op := range ops {
		wantNew, ok := want[op.oldPath]
		if !ok {
			t.Errorf("unexpected op for %q", op.oldPath)
			continue
		}
		if op.newPath != wantNew {
			t.Errorf("op.newPath = %q, want %q", op.newPath, wantNew)
		}
	}
}

func TestCollectRenames_NoMatch(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"foo.txt", "bar.txt"})

	re := mustCompile(t, `\.go$`)
	ops, err := collectRenames(dir, re, "renamed.go", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 0 {
		t.Errorf("got %d ops, want 0", len(ops))
	}
}

func TestCollectRenames_Recursive(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{
		"001_top.txt",
		"sub/001_child.txt",
		"sub/002_child.txt",
	})

	re := mustCompile(t, `(\d+)_`)
	ops, err := collectRenames(dir, re, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 {
		t.Fatalf("got %d ops, want 3", len(ops))
	}

	// Deeper paths must come before shallower ones.
	for i := 1; i < len(ops); i++ {
		if ops[i-1].depth < ops[i].depth {
			t.Errorf("ops not sorted deepest-first: ops[%d].depth=%d < ops[%d].depth=%d",
				i-1, ops[i-1].depth, i, ops[i].depth)
		}
	}
}

func TestCollectRenames_RecursiveDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"old_dir/", "old_dir/old_file.txt"})

	re := mustCompile(t, `^old_`)
	ops, err := collectRenames(dir, re, "new_", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Fatalf("got %d ops, want 2", len(ops))
	}
	// File (deeper) must come before directory (shallower).
	if ops[0].depth <= ops[1].depth {
		t.Errorf("file op should be deeper than dir op")
	}
}

func TestExecuteRenames(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"a.txt", "b.txt"})

	ops := []renameOp{
		{oldPath: filepath.Join(dir, "a.txt"), newPath: filepath.Join(dir, "a_renamed.txt")},
		{oldPath: filepath.Join(dir, "b.txt"), newPath: filepath.Join(dir, "b_renamed.txt")},
	}
	errs := executeRenames(ops)
	if len(errs) != 0 {
		t.Fatalf("executeRenames returned errors: %v", errs)
	}

	for _, op := range ops {
		if _, err := os.Stat(op.newPath); err != nil {
			t.Errorf("expected %q to exist after rename: %v", op.newPath, err)
		}
		if _, err := os.Stat(op.oldPath); !os.IsNotExist(err) {
			t.Errorf("expected %q to be gone after rename", op.oldPath)
		}
	}
}

func TestExecuteRenames_Error(t *testing.T) {
	dir := t.TempDir()
	ops := []renameOp{
		{oldPath: filepath.Join(dir, "nonexistent.txt"), newPath: filepath.Join(dir, "new.txt")},
	}
	errs := executeRenames(ops)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateRenames_Clean(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"a.txt", "b.txt"})

	ops := []renameOp{
		{oldPath: filepath.Join(dir, "a.txt"), newPath: filepath.Join(dir, "a_new.txt")},
		{oldPath: filepath.Join(dir, "b.txt"), newPath: filepath.Join(dir, "b_new.txt")},
	}
	if errs := validateRenames(ops); len(errs) != 0 {
		t.Errorf("expected no conflicts, got: %v", errs)
	}
}

func TestValidateRenames_DestinationExists(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"a.txt", "existing.txt"})

	ops := []renameOp{
		{oldPath: filepath.Join(dir, "a.txt"), newPath: filepath.Join(dir, "existing.txt")},
	}
	errs := validateRenames(ops)
	if len(errs) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(errs))
	}
}

func TestValidateRenames_DuplicateDestination(t *testing.T) {
	dir := t.TempDir()
	mkfiles(t, dir, []string{"a.txt", "b.txt"})

	ops := []renameOp{
		{oldPath: filepath.Join(dir, "a.txt"), newPath: filepath.Join(dir, "same.txt")},
		{oldPath: filepath.Join(dir, "b.txt"), newPath: filepath.Join(dir, "same.txt")},
	}
	errs := validateRenames(ops)
	if len(errs) != 1 {
		t.Fatalf("expected 1 conflict (duplicate dest), got %d", len(errs))
	}
}

func TestValidateRenames_DestIsSourceInBatch(t *testing.T) {
	// a.txt → b.txt, b.txt → c.txt: b.txt exists but is also being renamed → no conflict
	dir := t.TempDir()
	mkfiles(t, dir, []string{"a.txt", "b.txt"})

	ops := []renameOp{
		{oldPath: filepath.Join(dir, "a.txt"), newPath: filepath.Join(dir, "b.txt")},
		{oldPath: filepath.Join(dir, "b.txt"), newPath: filepath.Join(dir, "c.txt")},
	}
	if errs := validateRenames(ops); len(errs) != 0 {
		t.Errorf("expected no conflicts when dest is also a source, got: %v", errs)
	}
}
