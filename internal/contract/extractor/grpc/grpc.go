package grpc

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// GRPCExtractor extracts gRPC service contracts from .proto files
// Uses simple line-based parsing to avoid protobuf dependencies
type GRPCExtractor struct{}

// Name returns the extractor name
func (e *GRPCExtractor) Name() string {
	return "grpc"
}

// Kind returns the contract kind
func (e *GRPCExtractor) Kind() model.ContractKind {
	return model.ContractKindGRPCService
}

// Language returns the supported language
// Note: proto files are language-agnostic, but we use "go" as a placeholder
func (e *GRPCExtractor) Language() model.Language {
	return model.LanguageGo
}

// CanHandle checks if this file can be handled by the grpc extractor
func (e *GRPCExtractor) CanHandle(filePath string) bool {
	return filepath.Ext(filePath) == ".proto"
}

// Extract extracts gRPC service contracts from the proto file
func (e *GRPCExtractor) Extract(filePath string) ([]*model.Contract, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var contracts []*model.Contract
	scanner := bufio.NewScanner(file)

	// Regular expressions for parsing
	serviceRe := regexp.MustCompile(`^\s*service\s+(\w+)\s*\{`)
	rpcRe := regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(\w+)\s*\)`)

	var currentService string
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for service definition
		if matches := serviceRe.FindStringSubmatch(line); matches != nil {
			currentService = matches[1]
			continue
		}

		// Check for rpc method definition
		if matches := rpcRe.FindStringSubmatch(line); matches != nil && currentService != "" {
			methodName := matches[1]
			inputType := matches[2]
			outputType := matches[3]

			signature := map[string]interface{}{
				"service": currentService,
				"method":  methodName,
				"input":   inputType,
				"output":  outputType,
			}

			contract := &model.Contract{
				Kind:       model.ContractKindGRPCService,
				Identifier: fmt.Sprintf("%s.%s", currentService, methodName),
				File:       filePath,
				Line:       lineNum,
				Signature:  signature,
				Framework:  "grpc",
				Language:   model.LanguageGo,
			}

			if err := contract.ComputeSignatureHash(); err == nil {
				contracts = append(contracts, contract)
			}
		}

		// Check for end of service block
		if strings.TrimSpace(line) == "}" && currentService != "" {
			currentService = ""
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return contracts, nil
}

// IsEntryPoint checks if a symbol is a gRPC service entry point
func (e *GRPCExtractor) IsEntryPoint(symbol *model.Symbol) bool {
	if symbol == nil {
		return false
	}

	// gRPC service methods typically have "Server" in the type or name
	nameLower := strings.ToLower(symbol.Name)
	return strings.Contains(nameLower, "server") || strings.Contains(symbol.Signature, "grpc.Server")
}
