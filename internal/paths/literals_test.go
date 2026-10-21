package paths

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoPathLiterals fails when production code spells a task state path as a
// string literal instead of asking this package: a literal that is exactly
// "tasks" or ".taskfactory", or that starts with "tasks/" or ".taskfactory/".
// Prose that merely mentions a path, such as help text and error messages, does
// not start with one of those and is not flagged.
func TestNoPathLiterals(t *testing.T) {
	root := filepath.Join("..", "..")
	var found []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			switch rel {
			case ".git", ".taskfactory", "tasks", filepath.Join("internal", "paths"):
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if value == "tasks" || value == ".taskfactory" || strings.HasPrefix(value, "tasks/") || strings.HasPrefix(value, ".taskfactory/") {
				found = append(found, fset.Position(lit.Pos()).String()+": "+lit.Value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("hardcoded task path literal; use internal/paths: %s", f)
	}
}
