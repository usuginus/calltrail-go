package analyzer

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type sourceFile struct {
	path        string
	displayPath string
}

type parsedSource struct {
	sourceFile
	file                 *ast.File
	packageName          string
	fieldTypes           map[string]map[string]string
	stdlibPackageAliases map[string]bool
}

func loadSources(paths []string) (*token.FileSet, []parsedSource, error) {
	var files []sourceFile
	for _, path := range paths {
		matched, err := collectGoFiles(path)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, matched...)
	}

	fset := token.NewFileSet()
	sources := make([]parsedSource, 0, len(files))
	for _, file := range files {
		parsedFile, err := parser.ParseFile(fset, file.path, nil, 0)
		if err != nil {
			return nil, nil, err
		}
		sources = append(sources, parsedSource{
			sourceFile:           file,
			file:                 parsedFile,
			packageName:          parsedFile.Name.Name,
			fieldTypes:           collectStructFieldTypes(parsedFile),
			stdlibPackageAliases: collectStdlibPackageAliases(parsedFile),
		})
	}
	return fset, sources, nil
}

func collectGoFiles(root string) ([]sourceFile, error) {
	root = strings.TrimSuffix(root, "/...")
	if root == "" {
		root = "."
	}
	repoRoot := findRepoRoot(root)
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, sourceFile{
				path:        path,
				displayPath: displayPath(repoRoot, path),
			})
		}
		return nil
	})
	return files, err
}

func findRepoRoot(path string) string {
	current := path
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	abs, err := filepath.Abs(current)
	if err != nil {
		abs = current
	}
	for {
		if exists(filepath.Join(abs, "go.mod")) || exists(filepath.Join(abs, ".git")) {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return current
		}
		abs = parent
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func displayPath(root, path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return filepath.ToSlash(path)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func collectStdlibPackageAliases(file *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || !isStdlibImportPath(importPath) {
			continue
		}
		if spec.Name != nil {
			switch spec.Name.Name {
			case ".", "_":
				continue
			default:
				aliases[spec.Name.Name] = true
				continue
			}
		}
		aliases[defaultImportName(importPath)] = true
	}
	return aliases
}

func isStdlibImportPath(importPath string) bool {
	pkg, err := build.Default.Import(importPath, "", build.FindOnly)
	return err == nil && pkg.Goroot
}

func defaultImportName(importPath string) string {
	if idx := strings.LastIndex(importPath, "/"); idx >= 0 {
		return importPath[idx+1:]
	}
	return importPath
}
