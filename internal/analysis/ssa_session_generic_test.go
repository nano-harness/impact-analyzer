package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSASession_GenericSourceIdentity(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":  "module example.com/genericcalls\n\ngo 1.25\n",
		"main.go": "package main\nfunc identity[T any](value T) T { return value }\nfunc main() { _ = identity(1) }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	session := NewSSASession(dir, nil, nil, false)
	reachable, err := session.Reachable()
	if err != nil {
		t.Fatal(err)
	}
	if !reachable[PosKey{File: filepath.Join(dir, "main.go"), Line: 2, Name: "identity"}] {
		t.Error("reachable generic function must use its source declaration name")
	}
	edges, err := session.CallEdgesForDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, edge := range edges {
		if strings.Contains(edge.CallerName, "[") || strings.Contains(edge.CalleeName, "[") {
			t.Errorf("edge leaked instantiation names instead of source names: %+v", edge)
		}
		if edge.CallerName == "main" && edge.CalleeName == "identity" {
			found = true
		}
	}
	if !found {
		t.Error("missing main -> identity source call edge")
	}
}
