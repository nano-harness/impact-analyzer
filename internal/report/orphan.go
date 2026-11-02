package report

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

const (
	defaultOrphanGroupLimit = 5
	topOrphanCandidateLimit = 10
)

type OrphanReportOptions struct {
	Limit   int
	All     bool
	GroupBy string
}

func GenerateOrphanReport(result *model.OrphanResult, format string) (string, error) {
	return GenerateOrphanReportWithOptions(result, format, OrphanReportOptions{All: true, GroupBy: "none"})
}

func GenerateOrphanReportWithOptions(result *model.OrphanResult, format string, opts OrphanReportOptions) (string, error) {
	switch strings.ToLower(format) {
	case "json":
		b, err := json.MarshalIndent(result, "", "  ")
		return string(b), err
	case "markdown", "md":
		return orphanMarkdown(result, normalizeOrphanReportOptions(opts)), nil
	case "text", "":
		return orphanText(result, normalizeOrphanReportOptions(opts)), nil
	default:
		return "", fmt.Errorf("unsupported orphan format: %s", format)
	}
}

func normalizeOrphanReportOptions(opts OrphanReportOptions) OrphanReportOptions {
	if opts.GroupBy == "" {
		opts.GroupBy = "kind"
	}
	if opts.GroupBy != "kind" && opts.GroupBy != "dir" && opts.GroupBy != "none" {
		opts.GroupBy = "kind"
	}
	if opts.Limit < 0 {
		opts.Limit = 0
	}
	return opts
}

func orphanText(result *model.OrphanResult, opts OrphanReportOptions) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Orphan candidates: %d (high: %d, medium: %d, low: %d)\n", result.Summary.Total, result.Summary.High, result.Summary.Medium, result.Summary.Low))
	// Render diagnostics
	for _, diag := range result.Diagnostics {
		switch diag.Level {
		case "warning":
			sb.WriteString(fmt.Sprintf("[WARNING] %s\n", diag.Message))
		case "error":
			sb.WriteString(fmt.Sprintf("[ERROR] %s\n", diag.Message))
		case "info":
			sb.WriteString(fmt.Sprintf("[INFO] %s\n", diag.Message))
		}
	}
	writeTextGroups(&sb, displayCandidates(result.Candidates, opts), opts)
	return sb.String()
}

func writeTextGroups(sb *strings.Builder, candidates []*model.OrphanCandidate, opts OrphanReportOptions) {
	for _, group := range groupCandidates(candidates, opts.GroupBy) {
		sb.WriteString("\n")
		sb.WriteString(group.Name)
		sb.WriteString("\n")
		shown := group.Items
		more := 0
		if !opts.All && opts.GroupBy != "none" && len(shown) > defaultOrphanGroupLimit {
			more = len(shown) - defaultOrphanGroupLimit
			shown = shown[:defaultOrphanGroupLimit]
		}
		for _, c := range shown {
			if c.Symbol == nil {
				continue
			}
			sym := c.Symbol
			sb.WriteString(fmt.Sprintf("- [%s] %s (%s:%d) score %.1f\n", c.Confidence, sym.QualifiedName(), sym.File, sym.Line, c.Score))
			for _, reason := range c.Reasons {
				sb.WriteString(fmt.Sprintf("  - %s\n", reason))
			}
		}
		if more > 0 {
			sb.WriteString(fmt.Sprintf("...and %d more (use --all to view)\n", more))
		}
	}
}

func orphanMarkdown(result *model.OrphanResult, opts OrphanReportOptions) string {
	var sb strings.Builder
	sb.WriteString("# Orphan Code Report\n\n")
	// Render diagnostics as blockquotes
	for _, diag := range result.Diagnostics {
		switch diag.Level {
		case "warning":
			sb.WriteString(fmt.Sprintf("> ⚠️ **Warning**: %s\n\n", diag.Message))
		case "error":
			sb.WriteString(fmt.Sprintf("> ❌ **Error**: %s\n\n", diag.Message))
		case "info":
			sb.WriteString(fmt.Sprintf("> ℹ️ %s\n\n", diag.Message))
		}
	}
	sb.WriteString(fmt.Sprintf("Total: **%d** (high: %d, medium: %d, low: %d)\n\n", result.Summary.Total, result.Summary.High, result.Summary.Medium, result.Summary.Low))
	writeTopCandidates(&sb, result.Candidates)
	sb.WriteString("## Candidates\n\n")
	for _, group := range groupCandidates(displayCandidates(result.Candidates, opts), opts.GroupBy) {
		if opts.GroupBy != "none" {
			sb.WriteString(fmt.Sprintf("### %s\n\n", group.Name))
		}
		sb.WriteString("| Confidence | Symbol | Location | Reasons |\n")
		sb.WriteString("|---|---|---|---|\n")
		shown := group.Items
		more := 0
		if !opts.All && opts.GroupBy != "none" && len(shown) > defaultOrphanGroupLimit {
			more = len(shown) - defaultOrphanGroupLimit
			shown = shown[:defaultOrphanGroupLimit]
		}
		for _, c := range shown {
			if c.Symbol == nil {
				continue
			}
			sb.WriteString(fmt.Sprintf("| %s | `%s` | `%s:%d` | %s |\n", c.Confidence, c.Symbol.QualifiedName(), c.Symbol.File, c.Symbol.Line, strings.Join(c.Reasons, "; ")))
		}
		if more > 0 {
			sb.WriteString(fmt.Sprintf("\n...and %d more (use --all to view)\n", more))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func writeTopCandidates(sb *strings.Builder, candidates []*model.OrphanCandidate) {
	top := make([]*model.OrphanCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || c.Symbol == nil || c.Confidence.Rank() < model.ConfidenceMedium.Rank() {
			continue
		}
		switch c.Symbol.Kind {
		case model.SymbolKindFunction, model.SymbolKindMethod, model.SymbolKindInterface:
			top = append(top, c)
		}
	}
	sort.SliceStable(top, func(i, j int) bool {
		if top[i].Score != top[j].Score {
			return top[i].Score > top[j].Score
		}
		return top[i].Symbol.QualifiedName() < top[j].Symbol.QualifiedName()
	})
	if len(top) > topOrphanCandidateLimit {
		top = top[:topOrphanCandidateLimit]
	}
	sb.WriteString("## Top 10 高优先级候选\n\n")
	if len(top) == 0 {
		sb.WriteString("No medium-or-higher function/method/interface candidates.\n\n")
		return
	}
	sb.WriteString("| Score | Confidence | Kind | Symbol | Location |\n")
	sb.WriteString("|---|---|---|---|---|\n")
	for _, c := range top {
		sym := c.Symbol
		sb.WriteString(fmt.Sprintf("| %.1f | %s | %s | `%s` | `%s:%d` |\n", c.Score, c.Confidence, sym.Kind, sym.QualifiedName(), sym.File, sym.Line))
	}
	sb.WriteString("\n")
}

type orphanCandidateGroup struct {
	Name  string
	Items []*model.OrphanCandidate
}

func groupCandidates(candidates []*model.OrphanCandidate, groupBy string) []orphanCandidateGroup {
	if groupBy == "none" {
		return []orphanCandidateGroup{{Name: "Candidates", Items: candidates}}
	}
	groups := map[string][]*model.OrphanCandidate{}
	for _, c := range candidates {
		if c == nil || c.Symbol == nil {
			continue
		}
		key := string(c.Symbol.Kind)
		if groupBy == "dir" {
			key = filepath.ToSlash(filepath.Dir(c.Symbol.File))
			if key == "." {
				key = "root"
			}
		}
		groups[key] = append(groups[key], c)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]orphanCandidateGroup, 0, len(names))
	for _, name := range names {
		out = append(out, orphanCandidateGroup{Name: name, Items: groups[name]})
	}
	return out
}

func displayCandidates(candidates []*model.OrphanCandidate, opts OrphanReportOptions) []*model.OrphanCandidate {
	if opts.All || opts.Limit == 0 || len(candidates) <= opts.Limit {
		return candidates
	}
	return candidates[:opts.Limit]
}
