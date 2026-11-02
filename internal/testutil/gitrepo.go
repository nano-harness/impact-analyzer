package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// MiniRepo is a temporary git repository for self-contained tests.
type MiniRepo struct {
	// Dir is the absolute path to the temporary git repository directory.
	Dir string
}

// NewMiniRepo initializes a temporary git repository with deterministic user config.
func NewMiniRepo(t testing.TB) *MiniRepo {
	t.Helper()

	dir := t.TempDir()
	repo := &MiniRepo{Dir: dir}
	repo.git(t, "init")
	repo.git(t, "config", "user.name", "Impact Test")
	repo.git(t, "config", "user.email", "impact-test@example.com")
	return repo
}

// WriteFile writes content under the repository root.
func (r *MiniRepo) WriteFile(t testing.TB, path, content string) {
	t.Helper()

	fullPath := filepath.Join(r.Dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", fullPath, err)
	}
}

// Delete removes a path under the repository root.
func (r *MiniRepo) Delete(t testing.TB, path string) {
	t.Helper()
	if err := os.Remove(filepath.Join(r.Dir, filepath.FromSlash(path))); err != nil {
		t.Fatalf("delete %s: %v", path, err)
	}
}

// Rename renames a path under the repository root.
func (r *MiniRepo) Rename(t testing.TB, from, to string) {
	t.Helper()
	toPath := filepath.Join(r.Dir, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(toPath), err)
	}
	if err := os.Rename(filepath.Join(r.Dir, filepath.FromSlash(from)), toPath); err != nil {
		t.Fatalf("rename %s to %s: %v", from, to, err)
	}
}

// Commit stages all files and creates a commit, returning its hash.
func (r *MiniRepo) Commit(t testing.TB, msg string) string {
	t.Helper()
	r.git(t, "add", "-A")
	r.git(t, "commit", "-m", msg)
	return r.git(t, "rev-parse", "HEAD")
}

// Git runs git in the mini repo and returns trimmed stdout.
func (r *MiniRepo) Git(t testing.TB, args ...string) string {
	t.Helper()
	return r.git(t, args...)
}

func (r *MiniRepo) git(t testing.TB, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
