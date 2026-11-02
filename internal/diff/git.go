package diff

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Git provides Git operations
type Git struct {
	repoPath string
}

// NewGit creates a new Git instance
func NewGit(repoPath string) *Git {
	return &Git{repoPath: repoPath}
}

// Diff returns the unified diff between two refs
func (g *Git) Diff(ref string) (string, error) {
	if strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("invalid git ref %q: refs must not start with '-'", ref)
	}
	// --relative makes reported paths relative to g.repoPath instead of the
	// git root, so downstream joins against repoPath stay correct when the
	// analysis root is a subdirectory of the git repository.
	cmd := exec.Command("git", "diff", "--relative", ref)
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}

// DiffWithChangedFiles runs a single `git diff` invocation and returns both
// the unified diff text and the list of changed files (derived from the
// patch's "diff --git" headers, b-side, matching ChangedFiles semantics).
func (g *Git) DiffWithChangedFiles(ref string) (string, []string, error) {
	diffText, err := g.Diff(ref)
	if err != nil {
		return "", nil, err
	}
	return diffText, changedFilesFromPatch(diffText), nil
}

// changedFilesFromPatch extracts the b-side path of every "diff --git" header.
// Paths with control/non-ASCII characters are C-style quoted by git; paths
// with spaces appear verbatim (see splitDiffHeaderPaths).
func changedFilesFromPatch(diffText string) []string {
	var files []string
	for _, line := range strings.Split(diffText, "\n") {
		rest, ok := strings.CutPrefix(line, "diff --git ")
		if !ok {
			continue
		}
		_, b := splitDiffHeaderPaths(rest)
		name := strings.TrimPrefix(b, "b/")
		if name != "" {
			files = append(files, name)
		}
	}
	return files
}

// splitDiffHeaderPaths splits the "a/path b/path" remainder of a diff header
// into its two paths. Git only C-quotes paths with control/non-ASCII
// characters, so unquoted paths may contain spaces: first look for a " b/"
// split where both halves are equal (the common non-rename case), otherwise
// split at the last " b/" (rename: b-side is the new name, matching
// ChangedFiles semantics).
func splitDiffHeaderPaths(s string) (string, string) {
	if strings.HasPrefix(s, `"`) {
		a, rest := scanDiffPath(s)
		b, _ := scanDiffPath(strings.TrimLeft(rest, " "))
		return a, b
	}
	rest := strings.TrimPrefix(s, "a/")
	for i := 0; ; {
		idx := strings.Index(rest[i:], " b/")
		if idx < 0 {
			break
		}
		idx += i
		if rest[:idx] == rest[idx+3:] {
			return "a/" + rest[:idx], "b/" + rest[idx+3:]
		}
		i = idx + 1
	}
	if idx := strings.LastIndex(rest, " b/"); idx >= 0 {
		return "a/" + rest[:idx], "b/" + rest[idx+3:]
	}
	return s, ""
}

// scanDiffPath consumes one path token (quoted or unquoted) from s and returns
// it together with the unconsumed remainder.
func scanDiffPath(s string) (string, string) {
	if strings.HasPrefix(s, `"`) {
		// C-style quoted: find the closing quote, honoring backslash escapes.
		for i := 1; i < len(s); i++ {
			switch s[i] {
			case '\\':
				i++
			case '"':
				if unquoted, err := strconv.Unquote(s[:i+1]); err == nil {
					return unquoted, s[i+1:]
				}
				return s[1:i], s[i+1:]
			}
		}
		return s, ""
	}
	if idx := strings.IndexByte(s, ' '); idx >= 0 {
		return s[:idx], s[idx:]
	}
	return s, ""
}

// ChangedFiles returns list of files changed in a diff
func (g *Git) ChangedFiles(ref string) ([]string, error) {
	if strings.HasPrefix(ref, "-") {
		return nil, fmt.Errorf("invalid git ref %q: refs must not start with '-'", ref)
	}
	cmd := exec.Command("git", "diff", "--name-only", "--relative", ref)
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// GetFileAtRef returns file content at a specific ref
func (g *Git) GetFileAtRef(filePath, ref string) ([]byte, error) {
	cmd := exec.Command("git", "show", ref+":"+filePath)
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git show failed: %w", err)
	}
	return output, nil
}

// GetHEAD returns the current HEAD commit SHA
func (g *Git) GetHEAD() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ResolveRef resolves a git ref (branch, tag, HEAD~1, etc.) to a commit SHA
func (g *Git) ResolveRef(ref string) (string, error) {
	if strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("invalid git ref %q: refs must not start with '-'", ref)
	}
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = g.repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
