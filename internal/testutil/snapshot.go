package testutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var tempPathRE = regexp.MustCompile(`/tmp/[^"[:space:]]+`)

// AssertJSONSnapshot compares any JSON-serializable value with testdata/snapshots/name.json.
// Set UPDATE_SNAPSHOTS=1 to rewrite snapshots.
func AssertJSONSnapshot(t testing.TB, name string, value any) {
	t.Helper()

	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot %s: %v", name, err)
	}
	raw = normalizeSnapshot(raw)

	path := filepath.Join("testdata", "snapshots", name+".json")
	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir snapshot dir: %v", err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatalf("write snapshot %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir snapshot dir: %v", err)
		}
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			t.Fatalf("write initial snapshot %s: %v", path, err)
		}
		t.Fatalf("created missing snapshot %s; re-run test to compare", path)
	}
	if err != nil {
		t.Fatalf("read snapshot %s: %v", path, err)
	}
	want = normalizeSnapshot(bytes.TrimSpace(want))
	if !bytes.Equal(want, raw) {
		t.Fatalf("snapshot %s mismatch\nwant:\n%s\n\ngot:\n%s", name, want, raw)
	}
}

func normalizeSnapshot(raw []byte) []byte {
	s := string(raw)
	s = strings.ReplaceAll(s, "\\/", "/")
	s = tempPathRE.ReplaceAllString(s, "<tmp>")
	return []byte(strings.TrimSpace(s))
}
