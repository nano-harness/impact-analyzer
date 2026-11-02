package cobra

import (
	"path/filepath"
	"testing"
)

func TestExtract_SSAFailureFallsBackToAST(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "cmd.go")
	writeFile(t, testFile, `package cmd

import "github.com/spf13/cobra"

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test command",
}
`)

	e := NewCobraExtractor()
	e.EnableSSA(true)
	contracts, err := e.Extract(testFile)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}
	if contracts[0].Identifier != "test" {
		t.Fatalf("identifier = %q, want %q", contracts[0].Identifier, "test")
	}
}

func TestExtract_SSAZeroContractsFallsBackToAST(t *testing.T) {
	ResetSSACache()
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "go.mod"), `module example.com/fallback

go 1.22

require github.com/spf13/cobra v1.8.0

replace github.com/spf13/cobra => ./cobra
`)
	writeFile(t, filepath.Join(tmpDir, "cobra", "go.mod"), "module github.com/spf13/cobra\n\ngo 1.22\n")
	writeFile(t, filepath.Join(tmpDir, "cobra", "cobra.go"), `package cobra

type Command struct {
	Use   string
	Short string
}
`)
	testFile := filepath.Join(tmpDir, "cmd.go")
	writeFile(t, testFile, `package fallback

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "zero",
		Short: "Fallback command",
	}
}
`)

	e := NewCobraExtractor()
	e.EnableSSA(true)
	contracts, err := e.Extract(testFile)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	if len(contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(contracts))
	}
	if contracts[0].Identifier != "zero" {
		t.Fatalf("identifier = %q, want %q", contracts[0].Identifier, "zero")
	}
}
