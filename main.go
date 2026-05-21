package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	recursive := flag.Bool("r", false, "recursively process subdirectories")
	dryRun := flag.Bool("n", false, "dry-run: show preview without prompting")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: renamer [-r] [-n] <pattern> <replacement> [directory]\n")
		fmt.Fprintf(os.Stderr, "       renamer [-r] [directory]   # interactive mode\n\n")
		fmt.Fprintf(os.Stderr, "  pattern      RE2 regular expression to match filenames\n")
		fmt.Fprintf(os.Stderr, "  replacement  replacement string (supports $1, $2, ... capture groups)\n")
		fmt.Fprintf(os.Stderr, "  directory    target directory (default: current directory)\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()

	// Interactive mode: no pattern/replacement given
	if len(args) < 2 {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		runInteractive(dir, *recursive)
		return
	}

	pattern := args[0]
	replacement := normalizeReplacement(args[1])
	dir := "."
	if len(args) >= 3 {
		dir = args[2]
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid pattern: %v\n", err)
		os.Exit(1)
	}

	ops, err := collectRenames(dir, re, replacement, *recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(ops) == 0 {
		fmt.Println("No files matched.")
		return
	}

	conflicts := validateRenames(ops)
	conflictPaths := make(map[string]string, len(conflicts))
	for _, c := range conflicts {
		conflictPaths[c.op.oldPath] = c.reason
	}

	fmt.Println("Preview:")

	for _, op := range ops {
		if reason, bad := conflictPaths[op.oldPath]; bad {
			fmt.Printf("  %s → %s  [ERROR: %s]\n", op.oldPath, op.newPath, reason)
		} else {
			fmt.Printf("  %s → %s\n", op.oldPath, op.newPath)
		}
	}

	if len(conflicts) > 0 {
		fmt.Fprintf(os.Stderr, "%d 件の競合があります。実行できません。\n", len(conflicts))
		os.Exit(1)
	}

	if *dryRun {
		return
	}

	fmt.Print("Proceed? [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if answer := strings.TrimSpace(scanner.Text()); answer != "y" && answer != "Y" {
		fmt.Println("Aborted.")
		return
	}

	for _, err := range executeRenames(ops) {
		fmt.Fprintln(os.Stderr, err)
	}
	for _, op := range ops {
		fmt.Printf("Renamed: %s → %s\n", op.oldPath, op.newPath)
	}
}
