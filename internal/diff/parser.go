package diff

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderRe = regexp.MustCompile(`^@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@`)

// FileDiff represents changes to a single file
type FileDiff struct {
	OldPath string
	NewPath string
	Hunks   []*Hunk
}

// Hunk represents a continuous block of changes
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []string
}

// AffectedLines returns the line numbers that were added or deleted in this hunk
func (h *Hunk) AffectedLines() (added, deleted []int) {
	newLine := h.NewStart
	oldLine := h.OldStart

	for _, line := range h.Lines {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// Header lines, skip
			continue
		case strings.HasPrefix(line, "+"):
			added = append(added, newLine)
			newLine++
		case strings.HasPrefix(line, "-"):
			deleted = append(deleted, oldLine)
			oldLine++
		default:
			// Context line (starts with space or nothing)
			newLine++
			oldLine++
		}
	}
	return
}

// ParseUnifiedDiff parses a unified diff format
func ParseUnifiedDiff(diffText string) ([]*FileDiff, error) {
	var fileDiffs []*FileDiff
	var currentFile *FileDiff
	var currentHunk *Hunk

	scanner := bufio.NewScanner(strings.NewReader(diffText))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git") {
			// Start of new file
			if currentFile != nil && currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, currentHunk)
			}
			if currentFile != nil {
				fileDiffs = append(fileDiffs, currentFile)
			}
			currentFile = &FileDiff{}
			currentHunk = nil

		} else if strings.HasPrefix(line, "--- ") {
			if currentFile != nil {
				path := strings.TrimPrefix(strings.TrimPrefix(line, "--- "), "a/")
				if path == "/dev/null" {
					path = ""
				}
				currentFile.OldPath = path
			}

		} else if strings.HasPrefix(line, "+++ ") {
			if currentFile != nil {
				path := strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
				if path == "/dev/null" {
					path = ""
				}
				currentFile.NewPath = path
			}

		} else if strings.HasPrefix(line, "rename from ") {
			if currentFile != nil {
				currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
			}

		} else if strings.HasPrefix(line, "rename to ") {
			if currentFile != nil {
				currentFile.NewPath = strings.TrimPrefix(line, "rename to ")
			}

		} else if strings.HasPrefix(line, "@@") {
			// Start of new hunk
			if currentHunk != nil && currentFile != nil {
				currentFile.Hunks = append(currentFile.Hunks, currentHunk)
			}

			matches := hunkHeaderRe.FindStringSubmatch(line)
			if len(matches) >= 5 {
				oldStart, _ := strconv.Atoi(matches[1])
				oldLines := 1
				if matches[2] != "" {
					oldLines, _ = strconv.Atoi(matches[2])
				}
				newStart, _ := strconv.Atoi(matches[3])
				newLines := 1
				if matches[4] != "" {
					newLines, _ = strconv.Atoi(matches[4])
				}

				currentHunk = &Hunk{
					OldStart: oldStart,
					OldLines: oldLines,
					NewStart: newStart,
					NewLines: newLines,
					Lines:    []string{},
				}
			}

		} else if currentHunk != nil {
			// Add line to current hunk
			currentHunk.Lines = append(currentHunk.Lines, line)
		}
	}

	// Add last hunk and file
	if currentHunk != nil && currentFile != nil {
		currentFile.Hunks = append(currentFile.Hunks, currentHunk)
	}
	if currentFile != nil {
		fileDiffs = append(fileDiffs, currentFile)
	}

	return fileDiffs, scanner.Err()
}
