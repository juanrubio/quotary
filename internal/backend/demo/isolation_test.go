package demo

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var forbiddenDemoImports = []string{
	"os", "os/exec", "net", "net/http", "io/ioutil", "path/filepath", "syscall", "golang.org/x/sys",
}

func externalImports(dir string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			for _, forbidden := range forbiddenDemoImports {
				if importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/") {
					violations = append(violations, path+" imports "+importPath)
				}
			}
		}
		return nil
	})
	return violations, err
}

func TestDemoProductionSourcesHaveNoExternalIO(t *testing.T) {
	violations, err := externalImports(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

func TestOfflineImportCheckDetectsViolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(path, []byte("package bad\nimport _ \"os\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	violations, err := externalImports(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %v", violations)
	}
}
