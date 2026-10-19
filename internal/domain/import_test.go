package domain

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func importViolations(dir string, forbidden []string) ([]string, error) {
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
			for _, prefix := range forbidden {
				if strings.Contains(importPath, prefix) {
					violations = append(violations, path+" imports "+importPath)
				}
			}
		}
		return nil
	})
	return violations, err
}

func TestImportBoundaries(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		dir       string
		forbidden []string
	}{
		{filepath.Join(root, "internal", "domain"), []string{"charm.land/bubbletea", "charm.land/lipgloss", "internal/tui", "internal/theme", "internal/backend"}},
		{filepath.Join(root, "internal", "backend"), []string{"charm.land/bubbletea", "charm.land/lipgloss", "internal/tui", "internal/theme"}},
		{filepath.Join(root, "internal", "backend", "codexbar"), []string{"charm.land/bubbletea", "charm.land/lipgloss", "internal/tui", "internal/theme"}},
		{filepath.Join(root, "internal", "tui"), []string{"internal/backend/codexbar"}},
	}
	for _, tc := range cases {
		violations, err := importViolations(tc.dir, tc.forbidden)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range violations {
			t.Error(violation)
		}
	}
}

func TestImportBoundaryDetectsViolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(path, []byte("package bad\nimport _ \"charm.land/bubbletea/v2\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	violations, err := importViolations(dir, []string{"charm.land/bubbletea"})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("violations = %v", violations)
	}
}
