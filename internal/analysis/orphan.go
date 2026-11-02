package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

type OrphanAnalyzer struct {
	graph      *graph.CallGraph
	cfg        config.OrphanConfig
	extractors []contract.ContractExtractor
	goReach    GoReachabilityProvider // Optional SSA reachability for Go
}

type OrphanOptions struct {
	MinConfidence  model.ConfidenceLevel
	ChangedFiles   []string
	ChangedRanges  map[string][][2]int // file -> []{startLine, endLine}
	IncludeTests   bool
	OnlyNewIslands bool // if true, only report orphans within changed ranges
}

const (
	tinyStructMaxLines    = 5
	shortFunctionMaxLines = 3
)

func NewOrphanAnalyzer(cg *graph.CallGraph, cfg config.OrphanConfig, extractors []contract.ContractExtractor) *OrphanAnalyzer {
	return &OrphanAnalyzer{graph: cg, cfg: cfg, extractors: extractors}
}

// WithGoReachability sets an optional Go SSA reachability provider
func (a *OrphanAnalyzer) WithGoReachability(provider GoReachabilityProvider) *OrphanAnalyzer {
	a.goReach = provider
	return a
}

func (a *OrphanAnalyzer) Analyze(opts OrphanOptions) *model.OrphanResult {
	if opts.MinConfidence == "" {
		opts.MinConfidence = model.ConfidenceMedium
	}
	builder := NewEntrySetBuilder(a.cfg, a.extractors)
	entries := builder.Build(a.graph)
	marker := NewReachabilityMarker(a.graph)
	reachable := marker.Mark(entries)
	triState := marker.MarkTriState(entries)

	// Compute Go SSA reachability once if provider is available
	var goReachable map[PosKey]bool
	ssaPrimary := false
	var diagnostics []model.OrphanDiagnostic
	if a.goReach != nil {
		var err error
		goReachable, err = a.goReach.Reachable()
		if err != nil {
			// Log warning but continue with tree-sitter only
			fmt.Fprintf(os.Stderr, "[orphan] SSA reachability disabled: %v\n", err)
			goReachable = nil
			diagnostics = append(diagnostics, model.OrphanDiagnostic{
				Level:   "warning",
				Message: "Go SSA deep analysis unavailable, precision degraded to tree-sitter mode",
				Detail:  err.Error(),
			})
		} else {
			ssaPrimary = true
			rootCount := 0
			if session, ok := a.goReach.(*GoDeepReachability); ok && session.session != nil {
				rootCount = session.session.RootCount()
			}
			diagnostics = append(diagnostics, model.OrphanDiagnostic{
				Level:   "info",
				Message: fmt.Sprintf("Go SSA/RTA analysis succeeded: %d roots, %d reachable functions", rootCount, len(goReachable)),
			})
		}
	}

	scorer := NewConfidenceScorer(a.cfg.Confidence, opts.ChangedFiles, a.cfg.HighGateMinCallers)
	var candidates []*model.OrphanCandidate
	for _, sym := range a.graph.Symbols() {
		if sym == nil || sym.File == "" {
			continue
		}
		if sym.IsTest && !opts.IncludeTests {
			continue
		}
		if a.isIgnored(sym.File) {
			continue
		}
		noiseFlags := a.noiseFlags(sym)
		qname := sym.QualifiedName()

		// SSA-primary mode for Go functions/methods: SSA is the authoritative source
		if ssaPrimary && sym.Language == model.LanguageGo &&
			(sym.Kind == model.SymbolKindFunction || sym.Kind == model.SymbolKindMethod) {
			key := PosKeyOf(sym.File, sym.Line, sym.Name)
			if goReachable[key] {
				continue // SSA says reachable → skip
			}
			// SSA says unreachable, but tree-sitter StrongReach → keep as reachable
			// (covers linkname, CGo, and other edge cases SSA may miss)
			if state, ok := triState[qname]; ok && state == StrongReach {
				if !sym.IsExported {
					continue
				}
			}
			// Both SSA and tree-sitter agree: unreachable → high confidence candidate
			noiseFlags.SSAConfirmedOrphan = true
		} else {
			// Non-Go symbols or non-SSA mode: use original tri-state gating

			// Use tri-state: StrongReach symbols are definitely not orphans
			if state, ok := triState[qname]; ok && state == StrongReach {
				continue
			}

			// Legacy boolean reachability still gates (covers edge cases)
			if reachable[qname] {
				// If tri-state says WeakReach, still consider as candidate
				if state, ok := triState[qname]; !ok || state != WeakReach {
					continue
				}
			}

			// Check SSA reachability for Go functions/methods (non-primary fallback)
			if goReachable != nil &&
				sym.Language == model.LanguageGo &&
				(sym.Kind == model.SymbolKindFunction || sym.Kind == model.SymbolKindMethod) {
				key := PosKeyOf(sym.File, sym.Line, sym.Name)
				if goReachable[key] {
					continue
				}
			}
		}

		// Determine reach state for scoring
		reachState := Unreachable
		if state, ok := triState[qname]; ok {
			reachState = state
		}

		candidate := scorer.Score(sym, a.graph, onlyTestCallers(a.graph.GetCallers(sym)), noiseFlags, reachState)
		if candidate.Confidence.Rank() >= opts.MinConfidence.Rank() {
			candidates = append(candidates, candidate)
		}
	}

	// Apply OnlyNewIslands filter: restrict to candidates within changed ranges
	if opts.OnlyNewIslands && len(opts.ChangedRanges) > 0 {
		filtered := make([]*model.OrphanCandidate, 0, len(candidates))
		for _, c := range candidates {
			file := filepath.ToSlash(c.Symbol.File)
			ranges, ok := matchChangedRanges(file, opts.ChangedRanges)
			if !ok {
				continue
			}
			for _, rng := range ranges {
				if c.Symbol.Line >= rng[0] && c.Symbol.Line <= rng[1] {
					filtered = append(filtered, c)
					break
				}
			}
		}
		candidates = filtered
	} else if opts.OnlyNewIslands && len(opts.ChangedFiles) > 0 && len(opts.ChangedRanges) == 0 {
		// Fallback: if only ChangedFiles provided, filter to those files
		changedSet := make(map[string]bool, len(opts.ChangedFiles))
		for _, f := range opts.ChangedFiles {
			changedSet[filepath.ToSlash(f)] = true
		}
		filtered := make([]*model.OrphanCandidate, 0, len(candidates))
		for _, c := range candidates {
			file := filepath.ToSlash(c.Symbol.File)
			if matchesChangedFile(file, changedSet) {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Confidence != candidates[j].Confidence {
			return candidates[i].Confidence.Rank() > candidates[j].Confidence.Rank()
		}
		if candidates[i].Symbol.File != candidates[j].Symbol.File {
			return candidates[i].Symbol.File < candidates[j].Symbol.File
		}
		return candidates[i].Symbol.Line < candidates[j].Symbol.Line
	})
	result := &model.OrphanResult{SchemaVersion: "1.0", GeneratedAt: time.Now().Format(time.RFC3339), Candidates: candidates, EntryPoints: entrySlice(entries), Diagnostics: diagnostics}
	for _, c := range candidates {
		result.Summary.Total++
		switch c.Confidence {
		case model.ConfidenceHigh:
			result.Summary.High++
		case model.ConfidenceMedium:
			result.Summary.Medium++
		case model.ConfidenceLow:
			result.Summary.Low++
		}
	}
	result.Clusters = buildClusters(candidates)
	return result
}

func entrySlice(entries map[string]*model.Symbol) []*model.Symbol {
	out := make([]*model.Symbol, 0, len(entries))
	for _, sym := range entries {
		out = append(out, sym)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].QualifiedName() < out[j].QualifiedName() })
	return out
}

func ConfidenceFromString(value string) model.ConfidenceLevel { return confidenceFromString(value) }

func onlyTestCallers(callers []*model.Symbol) bool {
	if len(callers) == 0 {
		return false
	}
	for _, caller := range callers {
		if caller == nil || !caller.IsTest {
			return false
		}
	}
	return true
}

func (a *OrphanAnalyzer) isNoise(sym *model.Symbol) bool {
	return a.noiseFlags(sym) != NoiseFlags{}
}

func (a *OrphanAnalyzer) noiseFlags(sym *model.Symbol) NoiseFlags {
	nf := a.cfg.NoiseFilter
	return NoiseFlags{
		IsLocalVar:        nf.SkipLocalVariables && sym.Kind == model.SymbolKindVariable && !isPackageLevel(sym),
		IsUnexportedConst: nf.SkipUnexportedConstants && sym.Kind == model.SymbolKindConstant && !sym.IsExported,
		IsTinyStruct:      nf.SkipTinyStructs && sym.Kind == model.SymbolKindStruct && (sym.EndLine-sym.Line) < tinyStructMaxLines && !sym.IsExported,
		IsShortFunc:       nf.SkipShortFunctions && (sym.Kind == model.SymbolKindFunction || sym.Kind == model.SymbolKindMethod) && (sym.EndLine-sym.Line) < shortFunctionMaxLines,
	}
}

func isPackageLevel(sym *model.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Column == 1 {
		return true
	}
	line := symbolLine(sym)
	if line == "" {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if sym.Kind == model.SymbolKindVariable && strings.Contains(trimmed, ":=") {
		return false
	}
	if line != strings.TrimLeft(line, " \t") {
		return false
	}
	for _, prefix := range packageLevelPrefixes(sym.Language) {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func packageLevelPrefixes(language model.Language) []string {
	switch language {
	case model.LanguageGo:
		return []string{"var ", "const ", "type "}
	case model.LanguageJava:
		return []string{"class ", "interface ", "enum ", "public class ", "public interface "}
	case model.LanguageJavaScript:
		return []string{"var ", "let ", "const ", "class ", "function ", "export "}
	case model.LanguageTypeScript:
		return []string{"var ", "let ", "const ", "class ", "function ", "interface ", "type ", "export "}
	case model.LanguagePython:
		return []string{"class ", "def ", "async def "}
	case model.LanguageRust:
		return []string{"fn ", "pub ", "static ", "const ", "struct ", "enum ", "trait ", "mod ", "type "}
	default:
		return nil
	}
}

func symbolLine(sym *model.Symbol) string {
	if sym == nil || sym.File == "" || sym.Line <= 0 {
		return ""
	}
	content, err := os.ReadFile(sym.File)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	if sym.Line > len(lines) {
		return ""
	}
	return lines[sym.Line-1]
}

func (a *OrphanAnalyzer) isIgnored(file string) bool {
	file = filepath.ToSlash(file)
	for _, pattern := range a.cfg.Ignore {
		pattern = filepath.ToSlash(pattern)
		if matched, _ := doublestar.PathMatch(pattern, file); matched {
			return true
		}
		if matched, _ := doublestar.PathMatch(pattern, filepath.Base(file)); matched {
			return true
		}
	}
	return false
}

// matchChangedRanges looks up the changed line ranges for a candidate file path.
func matchChangedRanges(file string, changedRanges map[string][][2]int) ([][2]int, bool) {
	if ranges, ok := changedRanges[file]; ok {
		return ranges, true
	}
	for changedFile, r := range changedRanges {
		if strings.HasSuffix(file, "/"+changedFile) || file == changedFile {
			return r, true
		}
	}
	return nil, false
}

// matchesChangedFile checks whether a candidate file matches any entry in the changed set.
func matchesChangedFile(file string, changedSet map[string]bool) bool {
	if changedSet[file] {
		return true
	}
	for changed := range changedSet {
		if strings.HasSuffix(file, "/"+changed) {
			return true
		}
	}
	return false
}

// buildClusters groups orphan candidates by package and file prefix.
func buildClusters(candidates []*model.OrphanCandidate) []*model.OrphanCluster {
	if len(candidates) == 0 {
		return nil
	}

	type clusterKey struct {
		pkg        string
		filePrefix string
	}
	groups := make(map[clusterKey]*model.OrphanCluster)

	for _, c := range candidates {
		pkg := c.Symbol.Package
		if pkg == "" {
			pkg = filepath.Dir(filepath.ToSlash(c.Symbol.File))
		}
		fp := filePrefix(c.Symbol.File)
		key := clusterKey{pkg: pkg, filePrefix: fp}
		cluster, ok := groups[key]
		if !ok {
			cluster = &model.OrphanCluster{
				Package:    pkg,
				FilePrefix: fp,
			}
			groups[key] = cluster
		}
		cluster.Count++
		cluster.Symbols = append(cluster.Symbols, c.Symbol.QualifiedName())
		if c.Confidence.Rank() > cluster.Confidence.Rank() {
			cluster.Confidence = c.Confidence
		}
	}

	result := make([]*model.OrphanCluster, 0, len(groups))
	for _, cluster := range groups {
		result = append(result, cluster)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Package < result[j].Package
	})
	return result
}

// filePrefix returns the file path without _test.go suffix for clustering.
func filePrefix(file string) string {
	file = filepath.ToSlash(file)
	base := filepath.Base(file)
	if strings.HasSuffix(base, "_test.go") {
		base = strings.TrimSuffix(base, "_test.go") + ".go"
	}
	dir := filepath.Dir(file)
	if dir == "." {
		return base
	}
	return dir + "/" + base
}
