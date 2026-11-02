package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// contentStubMarker is a sentinel that no real extractor matches, so the stub
// below is the only extractor producing contracts for the test file.
const contentStubMarker = "impact-contract-content-stub:v1"

// countingContentExtractor is a test stub implementing ContentExtractor. It
// records which entry points BuildIndex used so tests can assert that the
// path-based (re-reading) methods are never called during index builds.
type countingContentExtractor struct {
	pathCanHandleCalls    int
	pathExtractCalls      int
	contentCanHandleCalls int
	contentExtractCalls   int
	sawContent            []byte
}

func (e *countingContentExtractor) Name() string             { return "test-content-stub" }
func (e *countingContentExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *countingContentExtractor) Language() model.Language { return model.LanguageGo }

func (e *countingContentExtractor) CanHandle(filePath string) bool {
	e.pathCanHandleCalls++
	content, err := os.ReadFile(filePath)
	return err == nil && strings.Contains(string(content), contentStubMarker)
}

func (e *countingContentExtractor) Extract(filePath string) ([]*model.Contract, error) {
	e.pathExtractCalls++
	return e.stubContracts(filePath), nil
}

func (e *countingContentExtractor) CanHandleContent(filePath string, content []byte) bool {
	e.contentCanHandleCalls++
	return strings.Contains(string(content), contentStubMarker)
}

func (e *countingContentExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	e.contentExtractCalls++
	e.sawContent = content
	return e.stubContracts(filePath), nil
}

func (e *countingContentExtractor) stubContracts(filePath string) []*model.Contract {
	return []*model.Contract{{
		Kind:       model.ContractKindHTTPRoute,
		Identifier: "STUB /",
		File:       filePath,
		Signature:  map[string]interface{}{"handler": "stubHandler"},
		Framework:  "test-content-stub",
		Language:   model.LanguageGo,
	}}
}

func (e *countingContentExtractor) IsEntryPoint(symbol *model.Symbol) bool { return false }

// pathOnlyExtractor is a test stub that does NOT implement ContentExtractor;
// BuildIndex must keep working with it through the legacy path-based calls.
type pathOnlyExtractor struct {
	canHandleCalls int
	extractCalls   int
}

func (e *pathOnlyExtractor) Name() string             { return "test-path-only-stub" }
func (e *pathOnlyExtractor) Kind() model.ContractKind { return model.ContractKindHTTPRoute }
func (e *pathOnlyExtractor) Language() model.Language { return model.LanguageGo }

func (e *pathOnlyExtractor) CanHandle(filePath string) bool {
	e.canHandleCalls++
	content, err := os.ReadFile(filePath)
	return err == nil && strings.Contains(string(content), contentStubMarker)
}

func (e *pathOnlyExtractor) Extract(filePath string) ([]*model.Contract, error) {
	e.extractCalls++
	return []*model.Contract{{
		Kind:       model.ContractKindHTTPRoute,
		Identifier: "STUB /path-only",
		File:       filePath,
		Signature:  map[string]interface{}{"handler": "pathOnlyHandler"},
		Framework:  "test-path-only-stub",
		Language:   model.LanguageGo,
	}}, nil
}

func (e *pathOnlyExtractor) IsEntryPoint(symbol *model.Symbol) bool { return false }

// TestBuildIndexReadsFileOncePerExtractor verifies that BuildIndex hands the
// single read of each file to extractors implementing ContentExtractor
// instead of letting every extractor re-read the file, while extractors
// without the content interface keep the legacy behavior.
func TestBuildIndexReadsFileOncePerExtractor(t *testing.T) {
	dir := t.TempDir()
	fileContent := contentStubMarker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(fileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	contentStub := &countingContentExtractor{}
	pathStub := &pathOnlyExtractor{}
	Register(contentStub)
	Register(pathStub)

	idx, err := BuildIndex(dir, nil)
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}

	// The content-aware stub must be served from the single read done by
	// BuildIndex: its legacy re-reading methods must never run.
	if contentStub.pathCanHandleCalls != 0 || contentStub.pathExtractCalls != 0 {
		t.Fatalf("path-based methods called on content extractor: CanHandle=%d Extract=%d, want 0",
			contentStub.pathCanHandleCalls, contentStub.pathExtractCalls)
	}
	if contentStub.contentCanHandleCalls == 0 {
		t.Fatal("CanHandleContent was never called")
	}
	if contentStub.contentExtractCalls != 1 {
		t.Fatalf("ExtractContent called %d times, want 1", contentStub.contentExtractCalls)
	}
	if string(contentStub.sawContent) != fileContent {
		t.Fatalf("ExtractContent saw %q, want the exact file content %q", contentStub.sawContent, fileContent)
	}

	// The path-only stub must still work via the legacy fallback.
	if pathStub.canHandleCalls == 0 || pathStub.extractCalls != 1 {
		t.Fatalf("path-only extractor fallback broken: CanHandle=%d Extract=%d, want >=1 and 1",
			pathStub.canHandleCalls, pathStub.extractCalls)
	}

	frameworks := map[string]bool{}
	for _, c := range idx.AllContracts() {
		frameworks[c.Framework] = true
	}
	if !frameworks["test-content-stub"] || !frameworks["test-path-only-stub"] {
		t.Fatalf("indexed frameworks = %v, want both stubs present", frameworks)
	}
}
