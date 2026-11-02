package lang

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// stubAdapter counts ParseFile invocations.
type stubAdapter struct {
	parseCalls   int
	callCalls    int
	resolveCalls int
}

func (a *stubAdapter) Language() model.Language { return model.LanguageGo }
func (a *stubAdapter) ParseFile(filePath string) (*model.FileSymbols, error) {
	a.parseCalls++
	return &model.FileSymbols{File: filePath, Language: model.LanguageGo}, nil
}
func (a *stubAdapter) ExtractCalls(filePath string) ([]*model.CallSite, error) {
	a.callCalls++
	return []*model.CallSite{{File: filePath, Line: 1}}, nil
}
func (a *stubAdapter) IsTestFile(filePath string) bool                        { return false }
func (a *stubAdapter) IsExported(symbolName string, context interface{}) bool { return true }
func (a *stubAdapter) ResolveImports(filePath string) ([]string, error) {
	a.resolveCalls++
	return []string{"dep"}, nil
}

func TestParseCacheMemoizesPerFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &stubAdapter{}
	cache := NewParseCache()

	for i := 0; i < 3; i++ {
		if _, err := cache.Parse(adapter, file); err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
	}
	if adapter.parseCalls != 1 {
		t.Fatalf("parseCalls=%d, want 1 (memoized)", adapter.parseCalls)
	}
}

func TestParseCacheInvalidatesOnContentChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &stubAdapter{}
	cache := NewParseCache()
	if _, err := cache.Parse(adapter, file); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Change size and mtime so the (path, mtime, size) key no longer matches.
	newer := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(file, []byte("package p\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(file, newer, newer); err != nil {
		t.Fatal(err)
	}

	if _, err := cache.Parse(adapter, file); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if adapter.parseCalls != 2 {
		t.Fatalf("parseCalls=%d, want 2 (changed file re-parsed)", adapter.parseCalls)
	}
}

func TestParseCacheClear(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &stubAdapter{}
	cache := NewParseCache()
	if _, err := cache.Parse(adapter, file); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	cache.Clear()
	if _, err := cache.Parse(adapter, file); err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if adapter.parseCalls != 2 {
		t.Fatalf("parseCalls=%d, want 2 after Clear", adapter.parseCalls)
	}
}

func TestParseCacheExtractCallsMemoized(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &stubAdapter{}
	cache := NewParseCache()

	for i := 0; i < 3; i++ {
		if _, err := cache.ExtractCalls(adapter, file); err != nil {
			t.Fatalf("ExtractCalls failed: %v", err)
		}
	}
	if adapter.callCalls != 1 {
		t.Fatalf("callCalls=%d, want 1 (memoized)", adapter.callCalls)
	}

	cache.Clear()
	if _, err := cache.ExtractCalls(adapter, file); err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}
	if adapter.callCalls != 2 {
		t.Fatalf("callCalls=%d, want 2 after Clear", adapter.callCalls)
	}
}

func TestParseCacheResolveImportsMemoizedAndCopied(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := &stubAdapter{}
	cache := NewParseCache()

	first, err := cache.ResolveImports(adapter, file)
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}
	first[0] = "mutated" // must not corrupt the cached entry

	second, err := cache.ResolveImports(adapter, file)
	if err != nil {
		t.Fatalf("ResolveImports failed: %v", err)
	}
	if adapter.resolveCalls != 1 {
		t.Fatalf("resolveCalls=%d, want 1 (memoized)", adapter.resolveCalls)
	}
	if len(second) != 1 || second[0] != "dep" {
		t.Fatalf("second=%v, want [dep] (caller mutation leaked into cache)", second)
	}
}
