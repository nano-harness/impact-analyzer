package analysis

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// LineRange represents a range of lines
type LineRange struct {
	Start int
	End   int
}

// LoadCoverageProfile parses a Go coverage profile and returns file -> covered line ranges
func LoadCoverageProfile(path string) (map[string][]LineRange, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open coverage file: %w", err)
	}
	defer file.Close()

	profile := make(map[string][]LineRange)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		// Parse coverage line format: file.go:startLine.startCol,endLine.endCol numStmt count
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		// Split file:lines
		fileParts := strings.Split(parts[0], ":")
		if len(fileParts) < 2 {
			continue
		}

		filePath := filepath.Clean(fileParts[0])

		// Parse line range: startLine.startCol,endLine.endCol
		lineRange := fileParts[1]
		rangeParts := strings.Split(lineRange, ",")
		if len(rangeParts) < 2 {
			continue
		}

		// Parse start line
		startParts := strings.Split(rangeParts[0], ".")
		if len(startParts) < 1 {
			continue
		}
		startLine, err := strconv.Atoi(startParts[0])
		if err != nil {
			continue
		}

		// Parse end line
		endParts := strings.Split(rangeParts[1], ".")
		if len(endParts) < 1 {
			continue
		}
		endLine, err := strconv.Atoi(endParts[0])
		if err != nil {
			continue
		}

		// Parse count (number of times this block was executed)
		count, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		// Only add covered ranges (count > 0)
		if count > 0 {
			profile[filePath] = append(profile[filePath], LineRange{
				Start: startLine,
				End:   endLine,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading coverage file: %w", err)
	}

	return profile, nil
}

// AnnotateSymbol annotates a symbol with coverage status based on the coverage profile
func AnnotateSymbol(sym *model.Symbol, profile map[string][]LineRange) {
	if sym == nil {
		return
	}

	// Normalize file path
	filePath := filepath.Clean(sym.File)

	// Get covered ranges for this file
	ranges, ok := profile[filePath]
	if !ok {
		// Try with relative path
		for profilePath, profileRanges := range profile {
			if strings.HasSuffix(profilePath, sym.File) || strings.HasSuffix(sym.File, profilePath) {
				ranges = profileRanges
				ok = true
				break
			}
		}
	}

	if !ok {
		// No coverage data for this file
		sym.CoverageStatus = model.CoverageStatusUncovered
		return
	}

	// Check if symbol lines are covered
	symbolStart := sym.Line
	symbolEnd := sym.EndLine
	if symbolEnd == 0 {
		symbolEnd = symbolStart
	}

	coveredLines := 0
	totalLines := symbolEnd - symbolStart + 1

	for _, r := range ranges {
		// Check overlap between symbol range and covered range
		overlapStart := max(symbolStart, r.Start)
		overlapEnd := min(symbolEnd, r.End)

		if overlapStart <= overlapEnd {
			coveredLines += overlapEnd - overlapStart + 1
		}
	}

	// Determine coverage status
	if coveredLines == 0 {
		sym.CoverageStatus = model.CoverageStatusUncovered
	} else if coveredLines >= totalLines {
		sym.CoverageStatus = model.CoverageStatusCovered
	} else {
		sym.CoverageStatus = model.CoverageStatusPartial
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
