package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// parseStubMarker is a sentinel that no real extractor matches, so only the
// stubs below produce contracts for the test file.
const parseStubMarker = "impact-contract-parse-stub:v1"

// parsingStubExtractor mimics the tree-sitter based extractors: its
// ExtractContent goes through astutil.ParseContent. It records the real parse
// count after each call so tests can assert how many parses actually ran.
type parsingStubExtractor struct {
	name             string
	parsesAfterCalls []int
}

func (e *parsingStubExtractor) Name() string             { return e.name }
func (e *parsingStubExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *parsingStubExtractor) Language() model.Language { return model.LanguageTypeScript }

func (e *parsingStubExtractor) CanHandle(filePath string) bool {
	content, err := os.ReadFile(filePath)
	return err == nil && e.CanHandleContent(filePath, content)
}

func (e *parsingStubExtractor) Extract(filePath string) ([]*model.Contract, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return e.ExtractContent(filePath, content)
}

func (e *parsingStubExtractor) CanHandleContent(filePath string, content []byte) bool {
	return strings.HasSuffix(filePath, ".ts") && strings.Contains(string(content), parseStubMarker)
}

func (e *parsingStubExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	if _, _, _, _, err := astutil.ParseContent(filePath, content); err != nil {
		return nil, err
	}
	e.parsesAfterCalls = append(e.parsesAfterCalls, astutil.RealParseCount())
	return []*model.Contract{{
		Kind:       model.ContractKindHTTPRoute,
		Identifier: "STUB " + e.name,
		File:       filePath,
		Signature:  map[string]interface{}{"handler": e.name + "Handler"},
		Framework:  e.name,
		Language:   model.LanguageTypeScript,
	}}, nil
}

func (e *parsingStubExtractor) IsEntryPoint(symbol *model.Symbol) bool { return false }

// TestBuildIndexParsesFileOnceAcrossExtractors registers two stub extractors
// that both parse the same .ts file through astutil and asserts the file is
// tree-sitter parsed only once per BuildIndex, with the cache lifecycle ended
// (and the counter reset) by the time BuildIndex returns.
func TestBuildIndexParsesFileOnceAcrossExtractors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "routes.ts"), []byte(parseStubMarker+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stubA := &parsingStubExtractor{name: "test-parse-stub-a"}
	stubB := &parsingStubExtractor{name: "test-parse-stub-b"}
	Register(stubA)
	Register(stubB)

	idx, err := BuildIndex(dir, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// Each stub extracted exactly once, and both observed the same parse
	// count of 1: the second ExtractContent reused the first one's parse.
	for _, stub := range []*parsingStubExtractor{stubA, stubB} {
		if len(stub.parsesAfterCalls) != 1 || stub.parsesAfterCalls[0] != 1 {
			t.Fatalf("%s observed parse counts %v, want exactly [1] (one parse shared by both extractors)",
				stub.name, stub.parsesAfterCalls)
		}
	}
	if n := astutil.RealParseCount(); n != 0 {
		t.Fatalf("RealParseCount after BuildIndex = %d, want 0 (cache cleared at end of build)", n)
	}

	frameworks := map[string]bool{}
	for _, c := range idx.AllContracts() {
		frameworks[c.Framework] = true
	}
	if !frameworks[stubA.name] || !frameworks[stubB.name] {
		t.Fatalf("indexed frameworks = %v, want both parse stubs present", frameworks)
	}
}
