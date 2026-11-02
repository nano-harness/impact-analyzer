package commander

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract/extractor/astutil"
	"github.com/nano-harness/impact-analyzer/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

type CommanderExtractor struct{}

type chainStep struct {
	method string
	args   *sitter.Node
	line   int
}

func (e *CommanderExtractor) Name() string             { return "commander" }
func (e *CommanderExtractor) Kind() model.ContractKind { return model.ContractKindCLICommand }
func (e *CommanderExtractor) Language() model.Language { return model.LanguageJavaScript }

func (e *CommanderExtractor) CanHandle(filePath string) bool {
	ext := filepath.Ext(filePath)
	if ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" {
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
func (e *CommanderExtractor) CanHandleContent(filePath string, content []byte) bool {
	ext := filepath.Ext(filePath)
	if ext != ".js" && ext != ".ts" && ext != ".jsx" && ext != ".tsx" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "commander") || strings.Contains(text, ".command(")
}

func (e *CommanderExtractor) Extract(filePath string) ([]*model.Contract, error) {
	return e.extract(filePath, nil)
}

// ExtractContent implements the content-based variant of Extract so index
// building can reuse a single read of the file across all extractors.
func (e *CommanderExtractor) ExtractContent(filePath string, content []byte) ([]*model.Contract, error) {
	return e.extract(filePath, content)
}

// extract parses the file with tree-sitter. A nil src makes astutil read the
// file from disk; otherwise src is used as the file content.
func (e *CommanderExtractor) extract(filePath string, src []byte) ([]*model.Contract, error) {
	_, tree, content, lang, err := astutil.ParseContent(filePath, src)
	if err != nil {
		return nil, err
	}
	// tree is shared and owned by astutil's parse cache; do not Close it
	// here, ClearParseCache releases it after each build/batch.

	var contracts []*model.Contract
	astutil.Walk(tree.RootNode(), func(node *sitter.Node) {
		if node.Kind() != "call_expression" || !astutil.IsOutermostCall(node) {
			return
		}
		_, steps := collectCommanderChain(node, content)
		for i, step := range steps {
			if step.method != "command" {
				continue
			}
			command, ok := astutil.StringValue(astutil.ArgNode(step.args, 0), content)
			if !ok || command == "" {
				continue
			}
			signature := map[string]interface{}{"command": command}
			var options []string
		collectMetadata:
			for _, next := range steps[i+1:] {
				switch next.method {
				case "option", "requiredOption":
					if value, ok := astutil.StringValue(astutil.ArgNode(next.args, 0), content); ok {
						options = append(options, value)
					}
				case "description":
					if value, ok := astutil.StringValue(astutil.ArgNode(next.args, 0), content); ok {
						signature["description"] = value
					}
				case "alias":
					if value, ok := astutil.StringValue(astutil.ArgNode(next.args, 0), content); ok {
						signature["alias"] = value
					}
				case "command":
					break collectMetadata
				}
			}
			if len(options) > 0 {
				signature["options"] = options
			}
			contract := &model.Contract{Kind: model.ContractKindCLICommand, Identifier: command, File: filePath, Line: step.line, Signature: signature, Framework: "commander", Language: lang}
			if err := contract.ComputeSignatureHash(); err == nil {
				contracts = append(contracts, contract)
			}
		}
	})
	return contracts, nil
}

func collectCommanderChain(call *sitter.Node, source []byte) (string, []chainStep) {
	var steps []chainStep
	current := call
	for current != nil && current.Kind() == "call_expression" {
		fn := current.ChildByFieldName("function")
		args := current.ChildByFieldName("arguments")
		if fn == nil || fn.Kind() != "member_expression" {
			if fn != nil {
				return astutil.Text(fn, source), steps
			}
			return "", steps
		}
		prop := fn.ChildByFieldName("property")
		obj := fn.ChildByFieldName("object")
		if prop == nil || obj == nil {
			break
		}
		steps = append([]chainStep{{method: astutil.Text(prop, source), args: args, line: int(prop.StartPosition().Row) + 1}}, steps...)
		if obj.Kind() == "call_expression" {
			current = obj
			continue
		}
		return astutil.Text(obj, source), steps
	}
	return "", steps
}

func (e *CommanderExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}
	return strings.HasSuffix(symbol.Name, "Command") || strings.Contains(strings.ToLower(symbol.Name), "command")
}
