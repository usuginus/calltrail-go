package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/usuginus/calltrail-go/internal/rules"
)

func writeNoResults(stderr io.Writer, opts Options, ruleSet rules.RuleSet) {
	fileCount := countGoFiles(opts.Paths)
	switch {
	case opts.RPC != "":
		fmt.Fprintf(stderr, "calltrail-go: no handlers matched --rpc %q\n", opts.RPC)
	default:
		fmt.Fprintln(stderr, "calltrail-go: no handlers found")
	}
	fmt.Fprintf(stderr, "  paths: %s\n", strings.Join(opts.Paths, ", "))
	fmt.Fprintf(stderr, "  scanned_go_files: %d\n", fileCount)
	if opts.Config == "" {
		fmt.Fprintln(stderr, "  rules: built-in generic")
	} else {
		fmt.Fprintf(stderr, "  rules: %s\n", opts.Config)
	}
	fmt.Fprintf(stderr, "  handler package_names: %s\n", listOrNone(ruleSet.Handlers.Match.PackageNames))
	fmt.Fprintf(stderr, "  handler file_path_contains: %s\n", listOrNone(ruleSet.Handlers.Match.FilePathContains))
	fmt.Fprintln(stderr, "Try:")
	fmt.Fprintf(stderr, "  calltrail-go %s --list\n", strings.Join(opts.Paths, " "))
	fmt.Fprintln(stderr, "  calltrail-go <path> --config .calltrail.yaml")
}

func listOrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

func countGoFiles(paths []string) int {
	count := 0
	for _, path := range paths {
		root := strings.TrimSuffix(path, "/...")
		if root == "" {
			root = "."
		}
		info, err := stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			if isGoSourceFile(root) {
				count++
			}
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "vendor":
					return filepath.SkipDir
				}
				return nil
			}
			if isGoSourceFile(path) {
				count++
			}
			return nil
		})
	}
	return count
}

func isGoSourceFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
