package clap

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

type ClapExtractor struct{}

type clapCommand struct {
	name string
	args []map[string]interface{}
	line int
}

var commandNameRE = regexp.MustCompile(`#\s*\[\s*command\s*\([^\]]*name\s*=\s*"([^"]+)"`)
var subcommandEnumRE = regexp.MustCompile(`(?s)#\s*\[\s*derive\s*\([^\]]*Subcommand[^\]]*\)\s*\]\s*(?:pub\s+)?enum\s+\w+\s*\{`)
var fieldRE = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([^,}]+)`)

func (e *ClapExtractor) Name() string             { return "clap" }
func (e *ClapExtractor) Kind() model.ContractKind { return model.ContractKindCLICommand }
func (e *ClapExtractor) Language() model.Language { return model.LanguageRust }

func (e *ClapExtractor) CanHandle(filePath string) bool {
	if filepath.Ext(filePath) != ".rs" {
		return false
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	return e.CanHandleContent(filePath, content)
}

// CanHandleContent implements the content-based variant of CanHandle so index
// building can reuse a single read of the file across all extractors.
func (e *ClapExtractor) CanHandleContent(filePath string, content []byte) bool {
	if filepath.Ext(filePath) != ".rs" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "clap") || strings.Contains(text, "derive(Subcommand)") || strings.Contains(text, "derive(Parser)")
}

func (e *ClapExtractor) Extract(filePath string) ([]*model.Contract, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return e.ExtractContent(filePath, content)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *ClapExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	text := string(content)
	root := rootCommandName(text)
	commands := extractSubcommands(text)
	contracts := make([]*model.Contract, 0, len(commands))
	seen := map[string]bool{}
	for _, cmd := range commands {
		identifier := cmd.name
		if root != "" {
			identifier = root + " " + cmd.name
		}
		if seen[identifier] {
			continue
		}
		seen[identifier] = true
		sig := map[string]interface{}{"name": cmd.name, "parent": root, "args": cmd.args}
		contract := &model.Contract{Kind: model.ContractKindCLICommand, Identifier: identifier, File: filePath, Line: cmd.line, Signature: sig, Framework: "clap", Language: model.LanguageRust}
		if err := contract.ComputeSignatureHash(); err == nil {
			contracts = append(contracts, contract)
		}
	}
	return contracts, nil
}

func rootCommandName(text string) string {
	if m := commandNameRE.FindStringSubmatch(text); len(m) == 2 {
		return m[1]
	}
	return ""
}

func extractSubcommands(text string) []clapCommand {
	var commands []clapCommand
	matches := subcommandEnumRE.FindAllStringIndex(text, -1)
	for _, match := range matches {
		open := match[1] - 1
		close := matchingBrace(text, open)
		if close < 0 {
			continue
		}
		body := text[open+1 : close]
		baseLine := 1 + strings.Count(text[:open], "\n")
		for _, part := range splitTopLevel(body, ',') {
			cmd, ok := parseVariant(part, baseLine+strings.Count(body[:strings.Index(body, part)], "\n"))
			if ok {
				commands = append(commands, cmd)
			}
		}
	}
	return commands
}

func parseVariant(raw string, line int) (clapCommand, bool) {
	text := stripRustAttributes(strings.TrimSpace(raw))
	if text == "" {
		return clapCommand{}, false
	}
	nameEnd := 0
	for nameEnd < len(text) {
		r := rune(text[nameEnd])
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			break
		}
		nameEnd++
	}
	if nameEnd == 0 {
		return clapCommand{}, false
	}
	name := toKebab(text[:nameEnd])
	cmd := clapCommand{name: name, line: line}
	if open := strings.Index(text, "{"); open >= 0 {
		if close := strings.LastIndex(text, "}"); close > open {
			cmd.args = parseFields(text[open+1 : close])
		}
	} else if open := strings.Index(text, "("); open >= 0 {
		if close := strings.LastIndex(text, ")"); close > open {
			typ := strings.TrimSpace(text[open+1 : close])
			if typ != "" {
				cmd.args = append(cmd.args, map[string]interface{}{"name": "args", "type": typ, "required": true})
			}
		}
	}
	if cmd.args == nil {
		cmd.args = []map[string]interface{}{}
	}
	return cmd, true
}

func stripRustAttributes(text string) string {
	for strings.HasPrefix(strings.TrimSpace(text), "#") {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "#[") {
			if end := strings.Index(text, "]"); end >= 0 {
				text = strings.TrimSpace(text[end+1:])
				continue
			}
		}
		break
	}
	return text
}

func parseFields(text string) []map[string]interface{} {
	cleaned := regexp.MustCompile(`#\s*\[[^\]]*\]`).ReplaceAllString(text, "")
	matches := fieldRE.FindAllStringSubmatch(cleaned, -1)
	args := make([]map[string]interface{}, 0, len(matches))
	for _, m := range matches {
		typ := strings.TrimSpace(m[2])
		args = append(args, map[string]interface{}{"name": m[1], "type": typ, "required": !strings.HasPrefix(typ, "Option<")})
	}
	return args
}

func splitTopLevel(text string, sep rune) []string {
	var parts []string
	start, curly, paren, bracket, angle := 0, 0, 0, 0, 0
	for i, r := range text {
		switch r {
		case '{':
			curly++
		case '}':
			curly--
		case '(':
			paren++
		case ')':
			paren--
		case '[':
			bracket++
		case ']':
			bracket--
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		default:
			if r == sep && curly == 0 && paren == 0 && bracket == 0 && angle == 0 {
				parts = append(parts, text[start:i])
				start = i + len(string(r))
			}
		}
	}
	parts = append(parts, text[start:])
	return parts
}

func matchingBrace(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func toKebab(name string) string {
	var b strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(unicode.ToLower(r))
		} else if r == '_' {
			b.WriteByte('-')
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func (e *ClapExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}
	name := strings.ToLower(symbol.Name)
	return strings.HasSuffix(name, "_command") || strings.HasSuffix(name, "_handler") || strings.Contains(name, "subcommand")
}
