package treesitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// TestParser_TSXGrammar verifies that .tsx files are parsed with the TSX
// grammar (JSX syntax) instead of being mis-parsed as plain TypeScript.
func TestParser_TSXGrammar(t *testing.T) {
	src := `export function Button({ label }: { label: string }) {
	return <button className="btn">{label}</button>;
}
`
	dir := t.TempDir()

	parser, err := NewParser(model.LanguageTypeScript)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}

	// .tsx file: must parse with the TSX grammar and contain JSX nodes
	tsxPath := filepath.Join(dir, "button.tsx")
	if err := os.WriteFile(tsxPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := parser.ParseFile(tsxPath)
	if err != nil {
		t.Fatalf("ParseFile(.tsx): %v", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("TSX parse produced ERROR nodes: %s", root.ToSexp())
	}
	if !strings.Contains(root.ToSexp(), "jsx_element") {
		t.Fatalf("expected jsx_element in TSX tree, got: %s", root.ToSexp())
	}

	// Queries on a TSX tree must compile against the TSX grammar
	matches, err := parser.Query(tree, "(jsx_element) @jsx")
	if err != nil {
		t.Fatalf("Query on TSX tree: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected jsx_element matches on TSX tree")
	}

	// Plain .ts file on the same parser still uses the TS grammar
	tsPath := filepath.Join(dir, "plain.ts")
	if err := os.WriteFile(tsPath, []byte("export const x: number = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tsTree, err := parser.ParseFile(tsPath)
	if err != nil {
		t.Fatalf("ParseFile(.ts): %v", err)
	}
	defer tsTree.Close()
	if tsTree.RootNode().HasError() {
		t.Fatalf("TS parse produced ERROR nodes: %s", tsTree.RootNode().ToSexp())
	}
	if _, err := parser.Query(tsTree, "(export_statement) @exp"); err != nil {
		t.Fatalf("Query on TS tree: %v", err)
	}
}
