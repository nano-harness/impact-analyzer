package diff

import (
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// IsCommentLine reports whether a trimmed changed line is a language comment.
func IsCommentLine(line string, _ model.Language) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*") ||
		strings.HasPrefix(trimmed, "#")
}

// IsWhitespaceOnly reports whether the line contains no non-whitespace text.
func IsWhitespaceOnly(line string) bool {
	return strings.TrimSpace(line) == ""
}

type hunkLine struct {
	kind string
	num  int
	text string
}

// FilterHunkChangedLines returns added/deleted line numbers after dropping
// comments, empty lines, and pure whitespace-only replacements.
func FilterHunkChangedLines(hunk *Hunk, language model.Language) (added, deleted []int, allFilteredOut bool) {
	newLine := hunk.NewStart
	oldLine := hunk.OldStart
	var changed []hunkLine

	for _, line := range hunk.Lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			changed = append(changed, hunkLine{kind: "+", num: newLine, text: strings.TrimPrefix(line, "+")})
			newLine++
		case strings.HasPrefix(line, "-"):
			changed = append(changed, hunkLine{kind: "-", num: oldLine, text: strings.TrimPrefix(line, "-")})
			oldLine++
		default:
			newLine++
			oldLine++
		}
	}

	if len(changed) == 0 {
		return nil, nil, true
	}

	filtered := make([]bool, len(changed))
	for i, line := range changed {
		if IsWhitespaceOnly(line.text) || IsCommentLine(line.text, language) {
			filtered[i] = true
		}
	}

	for i := 0; i+1 < len(changed); i++ {
		if filtered[i] || filtered[i+1] {
			continue
		}
		if changed[i].kind == "-" && changed[i+1].kind == "+" &&
			strings.TrimSpace(changed[i].text) == strings.TrimSpace(changed[i+1].text) {
			filtered[i] = true
			filtered[i+1] = true
		}
	}

	allFilteredOut = true
	for i, line := range changed {
		if filtered[i] {
			continue
		}
		allFilteredOut = false
		if line.kind == "+" {
			added = append(added, line.num)
		} else {
			deleted = append(deleted, line.num)
		}
	}
	return added, deleted, allFilteredOut
}
