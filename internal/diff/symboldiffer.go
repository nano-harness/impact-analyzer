package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// SymbolDiffer is the unified symbol-level diff implementation.
type SymbolDiffer struct {
	repoPath   string
	ref        string
	callGraph  *graph.CallGraph
	oldLoader  *OldFileLoader
	fallback   *OutOfFuncResolver
	parseCache *lang.ParseCache
}

func NewSymbolDiffer(repoPath, ref string, cg *graph.CallGraph) *SymbolDiffer {
	return &SymbolDiffer{
		repoPath:  repoPath,
		ref:       ref,
		callGraph: cg,
		oldLoader: NewOldFileLoader(repoPath, ref),
		fallback:  NewOutOfFuncResolver(cg),
	}
}

// SetParseCache attaches the orchestrator's per-run parse cache so worktree
// files already parsed for the call graph are not parsed again here.
func (d *SymbolDiffer) SetParseCache(c *lang.ParseCache) {
	d.parseCache = c
}

type affectedChange struct {
	symbol        *model.Symbol
	oldSymbol     *model.Symbol
	file          string
	addedLines    []int
	deletedLines  []int
	forceType     model.ChangeType
	outOfFunction bool
	fallbackKind  string
}

// Diff extracts de-noised symbol-level changes from parsed unified diffs.
func (d *SymbolDiffer) Diff(fileDiffs []*FileDiff) ([]*model.Change, error) {
	var changes []*model.Change
	affected := make(map[string]*affectedChange)

	for _, fd := range fileDiffs {
		fileChanges, err := d.diffFile(fd, affected)
		if err != nil {
			return nil, err
		}
		changes = append(changes, fileChanges...)
	}

	for _, aff := range affected {
		changeType := aff.forceType
		if changeType == "" {
			switch {
			case len(aff.addedLines) > 0 && len(aff.deletedLines) > 0:
				changeType = model.ChangeTypeModified
			case len(aff.addedLines) > 0:
				changeType = model.ChangeTypeAdded
			case len(aff.deletedLines) > 0:
				changeType = model.ChangeTypeDeleted
			default:
				changeType = model.ChangeTypeModified
			}
		}

		sym := aff.symbol
		if sym == nil {
			sym = aff.oldSymbol
		}
		change := &model.Change{
			Type:          changeType,
			Symbol:        sym,
			OldSymbol:     aff.oldSymbol,
			File:          aff.file,
			OutOfFunction: aff.outOfFunction,
			FallbackKind:  aff.fallbackKind,
		}
		if sym != nil {
			change.StartLine = sym.Line
			change.EndLine = sym.EndLine
		}
		lines := &fileLineCache{repoPath: d.repoPath, loader: d.oldLoader, newLines: map[string][]string{}, oldLines: map[string][]string{}}
		singleLineBodyOnly := d.singleLineBodyOnly(aff, lines)
		for _, line := range aff.addedLines {
			if IsLineInSignature(aff.symbol, line, nil) && !singleLineBodyOnly {
				change.SignatureChanged = true
			} else {
				change.BodyChanged = true
			}
		}
		for _, line := range aff.deletedLines {
			if (IsLineInSignature(aff.oldSymbol, line, nil) || IsLineInSignature(aff.symbol, line, nil)) && !singleLineBodyOnly {
				change.SignatureChanged = true
			} else {
				change.BodyChanged = true
			}
		}
		if changeType == model.ChangeTypeRenamed {
			change.SignatureChanged = false
			change.BodyChanged = false
		}
		changes = append(changes, change)
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].File != changes[j].File {
			return changes[i].File < changes[j].File
		}
		if changes[i].StartLine != changes[j].StartLine {
			return changes[i].StartLine < changes[j].StartLine
		}
		return changeName(changes[i]) < changeName(changes[j])
	})
	return changes, nil
}

func (d *SymbolDiffer) diffFile(fd *FileDiff, affected map[string]*affectedChange) ([]*model.Change, error) {
	if fd == nil {
		return nil, nil
	}
	if fd.OldPath != "" && fd.NewPath == "" {
		return nil, d.addWholeDeletedFile(fd, affected)
	}
	if fd.OldPath == "" && fd.NewPath != "" {
		return nil, d.addWholeAddedFile(fd, affected)
	}

	newSymbols, err := d.loadWorktreeSymbols(fd.NewPath)
	if err != nil {
		newSymbols = nil
	}

	var oldSymbols *model.FileSymbols
	if fd.OldPath != "" {
		if old, err := d.oldLoader.LoadOldSymbols(fd.OldPath); err == nil {
			oldSymbols = old
		}
	}

	if fd.OldPath != "" && fd.NewPath != "" && fd.OldPath != fd.NewPath && len(fd.Hunks) == 0 {
		d.addPureRename(fd, oldSymbols, newSymbols, affected)
		return nil, nil
	}

	var markers []*model.Change
	for _, hunk := range fd.Hunks {
		language := detectFileLanguage(fd.NewPath, fd.OldPath)
		added, deleted, allFilteredOut := FilterHunkChangedLines(hunk, language)
		if allFilteredOut {
			markers = append(markers, noiseMarker(fd, hunk))
			continue
		}
		for _, line := range added {
			d.addLineAffected(affected, fd.NewPath, newSymbols, line, true)
		}
		for _, line := range deleted {
			d.addLineAffected(affected, firstNonEmpty(fd.NewPath, fd.OldPath), oldSymbols, line, false)
		}
	}
	return markers, nil
}

func (d *SymbolDiffer) addWholeAddedFile(fd *FileDiff, affected map[string]*affectedChange) error {
	fileSymbols, err := d.loadWorktreeSymbols(fd.NewPath)
	if err != nil {
		return nil
	}
	for _, sym := range sortedSymbols(fileSymbols) {
		if isFileLevelSymbol(sym) {
			continue
		}
		aff := d.ensureAffected(affected, fd.NewPath, sym, nil)
		aff.forceType = model.ChangeTypeAdded
		aff.addedLines = append(aff.addedLines, sym.Line)
	}
	return nil
}

func (d *SymbolDiffer) addWholeDeletedFile(fd *FileDiff, affected map[string]*affectedChange) error {
	fileSymbols, err := d.oldLoader.LoadOldSymbols(fd.OldPath)
	if err != nil {
		return nil
	}
	for _, sym := range sortedSymbols(fileSymbols) {
		if isFileLevelSymbol(sym) {
			continue
		}
		aff := d.ensureAffected(affected, fd.OldPath, nil, sym)
		aff.forceType = model.ChangeTypeDeleted
		aff.deletedLines = append(aff.deletedLines, sym.Line)
	}
	return nil
}

func (d *SymbolDiffer) addPureRename(fd *FileDiff, oldSymbols, newSymbols *model.FileSymbols, affected map[string]*affectedChange) {
	oldByName := make(map[string]*model.Symbol)
	for _, sym := range sortedSymbols(oldSymbols) {
		if isFileLevelSymbol(sym) {
			continue
		}
		oldByName[sym.QualifiedName()] = sym
		oldByName[sym.Name] = sym
	}
	for _, sym := range sortedSymbols(newSymbols) {
		if isFileLevelSymbol(sym) {
			continue
		}
		oldSym := oldByName[sym.QualifiedName()]
		if oldSym == nil {
			oldSym = oldByName[sym.Name]
		}
		aff := d.ensureAffected(affected, fd.NewPath, sym, oldSym)
		aff.forceType = model.ChangeTypeRenamed
	}
}

func (d *SymbolDiffer) addLineAffected(affected map[string]*affectedChange, file string, fileSymbols *model.FileSymbols, line int, added bool) {
	sym := findEnclosingSymbol(fileSymbols, line)
	if sym == nil || !isFunctionLike(sym) {
		symbols, kind := d.fallback.Resolve(fileSymbols, line)
		if len(symbols) == 0 {
			key := fmt.Sprintf("%s@file-level:%d:%s", file, line, kind)
			if _, ok := affected[key]; !ok {
				affected[key] = &affectedChange{file: file, outOfFunction: true, fallbackKind: kind}
			}
			return
		}
		for _, fallbackSym := range symbols {
			aff := d.ensureAffected(affected, file, fallbackSym, nil)
			aff.outOfFunction = true
			aff.fallbackKind = kind
			if added {
				aff.addedLines = append(aff.addedLines, line)
			} else {
				aff.deletedLines = append(aff.deletedLines, line)
				aff.oldSymbol = fallbackSym
			}
		}
		return
	}

	if added {
		aff := d.ensureAffected(affected, file, sym, nil)
		aff.addedLines = append(aff.addedLines, line)
	} else {
		// Register the old-side symbol as oldSymbol only; passing it as sym
		// would overwrite aff.symbol (the new-side symbol set by added-line
		// processing), losing the reference needed for signature comparison.
		aff := d.ensureAffected(affected, file, nil, sym)
		aff.deletedLines = append(aff.deletedLines, line)
	}
}

func (d *SymbolDiffer) ensureAffected(affected map[string]*affectedChange, file string, sym, oldSym *model.Symbol) *affectedChange {
	keySym := sym
	if keySym == nil {
		keySym = oldSym
	}
	key := file + "@"
	if keySym != nil {
		// Key by kind/owner/name, not QualifiedName: the package component is
		// constant per file, and old-side symbols are parsed from a temp copy
		// whose path-derived package (e.g. Python module name) can differ from
		// the worktree parse. A package-insensitive key keeps old/new sides of
		// one modification in a single change entry.
		key += fmt.Sprintf("%s|%s|%s", keySym.Kind, keySym.Owner, keySym.Name)
	} else {
		key += "file-level"
	}
	aff, ok := affected[key]
	if !ok {
		aff = &affectedChange{symbol: sym, oldSymbol: oldSym, file: file}
		affected[key] = aff
	}
	if sym != nil {
		aff.symbol = sym
	}
	if oldSym != nil {
		aff.oldSymbol = oldSym
	}
	return aff
}

func (d *SymbolDiffer) loadWorktreeSymbols(path string) (*model.FileSymbols, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	fullPath := filepath.Join(d.repoPath, filepath.FromSlash(path))
	language := lang.DetectLanguage(fullPath)
	if language == model.LanguageUnknown {
		return nil, fmt.Errorf("unknown language for %s", path)
	}
	adapter, err := lang.Get(language)
	if err != nil {
		return nil, err
	}
	var fileSymbols *model.FileSymbols
	if d.parseCache != nil {
		// normalizeSymbolFiles mutates File fields below; copy the cached
		// entry first so the shared cache keeps its original values.
		cached, err := d.parseCache.Parse(adapter, fullPath)
		if err != nil {
			return nil, err
		}
		fileSymbols = cloneFileSymbols(cached)
	} else {
		fileSymbols, err = adapter.ParseFile(fullPath)
		if err != nil {
			return nil, err
		}
	}
	normalizeSymbolFiles(fileSymbols, path)
	return fileSymbols, nil
}

// cloneFileSymbols returns a copy of fileSymbols and its symbols so callers
// can mutate File fields without affecting shared cache entries.
func cloneFileSymbols(fileSymbols *model.FileSymbols) *model.FileSymbols {
	if fileSymbols == nil {
		return nil
	}
	cloned := *fileSymbols
	if fileSymbols.Symbols != nil {
		cloned.Symbols = make([]*model.Symbol, len(fileSymbols.Symbols))
		for i, sym := range fileSymbols.Symbols {
			if sym == nil {
				continue
			}
			symCopy := *sym
			cloned.Symbols[i] = &symCopy
		}
	}
	return &cloned
}

func normalizeSymbolFiles(fileSymbols *model.FileSymbols, path string) {
	if fileSymbols == nil {
		return
	}
	fileSymbols.File = path
	for _, sym := range fileSymbols.Symbols {
		sym.File = path
	}
}

func detectFileLanguage(paths ...string) model.Language {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if language := lang.DetectLanguage(path); language != model.LanguageUnknown {
			return language
		}
	}
	return model.LanguageUnknown
}

func noiseMarker(fd *FileDiff, hunk *Hunk) *model.Change {
	file := firstNonEmpty(fd.NewPath, fd.OldPath)
	endLine := hunk.NewStart
	if hunk.NewLines > 0 {
		endLine = hunk.NewStart + hunk.NewLines - 1
	}
	return &model.Change{
		Type:          model.ChangeTypeModified,
		File:          file,
		StartLine:     hunk.NewStart,
		EndLine:       endLine,
		CommentOnly:   true,
		NoiseFiltered: true,
	}
}

func isFunctionLike(sym *model.Symbol) bool {
	return sym != nil && (sym.Kind == model.SymbolKindFunction || sym.Kind == model.SymbolKindMethod)
}

func isFileLevelSymbol(sym *model.Symbol) bool {
	return sym == nil || sym.Kind == model.SymbolKindPackage || sym.Kind == model.SymbolKindModule
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func changeName(change *model.Change) string {
	if change.Symbol != nil {
		return change.Symbol.QualifiedName()
	}
	if change.OldSymbol != nil {
		return change.OldSymbol.QualifiedName()
	}
	return ""
}

// singleLineBodyOnly reports whether a change to a single-line symbol (whose
// declaration and body share one line) left the signature untouched. Line
// numbers alone cannot distinguish signature from body on such symbols, so
// the signature prefix (text before the body opener) of the old and new
// declaration is compared. Any uncertainty degrades to false, preserving the
// conservative line-based verdict.
func (d *SymbolDiffer) singleLineBodyOnly(aff *affectedChange, lines *fileLineCache) bool {
	sym, oldSym := aff.symbol, aff.oldSymbol
	if sym == nil || oldSym == nil {
		return false
	}
	if sym.Line != sym.EndLine || oldSym.Line != oldSym.EndLine {
		return false
	}
	newLine := lines.lineAt(false, aff.file, sym.Line)
	oldLine := lines.lineAt(true, aff.file, oldSym.Line)
	if newLine == "" || oldLine == "" {
		return false
	}
	newPrefix := signaturePrefix(newLine, sym.Language)
	oldPrefix := signaturePrefix(oldLine, oldSym.Language)
	if newPrefix == "" || oldPrefix == "" {
		return false
	}
	return newPrefix == oldPrefix
}

// signaturePrefix returns the whitespace-normalized declaration prefix of a
// single-line symbol: the text before the body opener ('{' for brace
// languages, ':' for Python). Returns "" when no opener is found.
func signaturePrefix(line string, language model.Language) string {
	opener := "{"
	if language == model.LanguagePython {
		opener = ":"
	}
	idx := strings.Index(line, opener)
	if idx < 0 {
		return ""
	}
	return strings.Join(strings.Fields(line[:idx]), " ")
}

// fileLineCache lazily loads worktree (new) and base-ref (old) file lines,
// cached per file, for single-line signature comparison.
type fileLineCache struct {
	repoPath string
	loader   *OldFileLoader
	newLines map[string][]string
	oldLines map[string][]string
}

// lineAt returns the 1-based line of file's content (old side = base ref,
// new side = worktree), or "" when unavailable.
func (c *fileLineCache) lineAt(old bool, file string, line int) string {
	if file == "" || line <= 0 {
		return ""
	}
	sides := c.newLines
	if old {
		sides = c.oldLines
	}
	lines, ok := sides[file]
	if !ok {
		lines = nil
		var data []byte
		var err error
		if old {
			data, err = c.loader.LoadOldContent(file)
		} else {
			data, err = os.ReadFile(filepath.Join(c.repoPath, filepath.FromSlash(file)))
		}
		if err == nil {
			lines = strings.Split(string(data), "\n")
		}
		sides[file] = lines
	}
	if line > len(lines) {
		return ""
	}
	return lines[line-1]
}
