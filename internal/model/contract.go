package model

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// ContractKind represents the type of public contract
type ContractKind string

const (
	ContractKindCLICommand     ContractKind = "cli_command"
	ContractKindHTTPRoute      ContractKind = "http_route"
	ContractKindRPCMethod      ContractKind = "rpc_method"
	ContractKindGRPCService    ContractKind = "grpc_service"
	ContractKindExportedSymbol ContractKind = "exported_symbol"
	ContractKindFrontendRoute  ContractKind = "frontend_route"
	ContractKindScheduledJob   ContractKind = "scheduled_job"
)

// Contract represents a public API contract (CLI command, HTTP route, RPC method, etc.)
type Contract struct {
	// Core identification
	Kind       ContractKind `json:"kind"`
	Identifier string       `json:"identifier"` // e.g., "app artifact deploy", "/api/v1/users", "UserService.GetUser"
	File       string       `json:"file"`
	Line       int          `json:"line"`

	// Signature represents the detailed contract specification
	// Structure varies by kind:
	// - CLI: {use, short, long, aliases, flags: [{name, type, default, usage, shorthand}],
	//         handlers: [{flag, callee, arg_index, depth}]}
	//   handlers tracks downstream business functions that consume each flag value.
	//   Including handlers in SignatureHash means handler refactoring (rename/move/
	//   parameter reorder) will be detected as contract modification.
	// - HTTP: {method, path, handler, middleware, params: [{name, type, required}]}
	// - RPC/gRPC: {service, method, input, output, options}
	// - Exported: {name, signature, doc}
	Signature map[string]interface{} `json:"signature"`

	// SignatureHash is SHA256 of stable JSON representation of Signature
	// Used for modified detection
	SignatureHash string `json:"signature_hash"`

	// Metadata
	Framework string   `json:"framework,omitempty"` // e.g., "cobra", "gin", "grpc", "express"
	Language  Language `json:"language"`

	// CausingChanges lists which changed methods caused this contract to be affected
	// (reverse trace: populated in surface analysis)
	CausingChanges []*Symbol `json:"causing_changes,omitempty"`
}

// ContractDiff represents changes in public contracts between two versions
type ContractDiff struct {
	ContractsAdded    []*Contract `json:"contracts_added"`
	ContractsRemoved  []*Contract `json:"contracts_removed"`
	ContractsModified []*Contract `json:"contracts_modified"` // New version of modified contracts
}

// IsEmpty returns true if the diff contains no changes
func (cd *ContractDiff) IsEmpty() bool {
	return len(cd.ContractsAdded) == 0 &&
		len(cd.ContractsRemoved) == 0 &&
		len(cd.ContractsModified) == 0
}

// Summary returns a human-readable summary of the diff
func (cd *ContractDiff) Summary() string {
	return fmt.Sprintf("Added: %d, Removed: %d, Modified: %d",
		len(cd.ContractsAdded), len(cd.ContractsRemoved), len(cd.ContractsModified))
}

// HashSignature computes a stable SHA256 hash of the contract signature
// The signature is serialized to JSON with sorted keys for stability
func HashSignature(signature map[string]interface{}) (string, error) {
	// Serialize to JSON with sorted keys
	jsonBytes, err := json.Marshal(sortedMap(signature))
	if err != nil {
		return "", fmt.Errorf("failed to marshal signature: %w", err)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256(jsonBytes)
	return fmt.Sprintf("%x", hash), nil
}

// sortedMap recursively sorts all maps in the structure for stable JSON output
func sortedMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))

	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = sortedMap(val)
		case []interface{}:
			result[k] = sortedSlice(val)
		default:
			result[k] = v
		}
	}

	return result
}

// sortedSlice recursively sorts maps within slices
func sortedSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))

	for i, v := range s {
		switch val := v.(type) {
		case map[string]interface{}:
			result[i] = sortedMap(val)
		case []interface{}:
			result[i] = sortedSlice(val)
		default:
			result[i] = v
		}
	}

	return result
}

// ComputeSignatureHash computes and sets the signature hash for a contract
func (c *Contract) ComputeSignatureHash() error {
	hash, err := HashSignature(c.Signature)
	if err != nil {
		return err
	}
	c.SignatureHash = hash
	return nil
}

// ContractsByHash creates a map of contracts indexed by their signature hash
func ContractsByHash(contracts []*Contract) map[string]*Contract {
	result := make(map[string]*Contract, len(contracts))
	for _, c := range contracts {
		if c.SignatureHash != "" {
			result[c.SignatureHash] = c
		}
	}
	return result
}

// ContractsByIdentifier creates a map of contracts indexed by their identifier
func ContractsByIdentifier(contracts []*Contract) map[string][]*Contract {
	result := make(map[string][]*Contract)
	for _, c := range contracts {
		result[c.Identifier] = append(result[c.Identifier], c)
	}
	return result
}

// SortContracts sorts contracts by kind, then identifier, then file
func SortContracts(contracts []*Contract) {
	sort.Slice(contracts, func(i, j int) bool {
		if contracts[i].Kind != contracts[j].Kind {
			return contracts[i].Kind < contracts[j].Kind
		}
		if contracts[i].Identifier != contracts[j].Identifier {
			return contracts[i].Identifier < contracts[j].Identifier
		}
		return contracts[i].File < contracts[j].File
	})
}
