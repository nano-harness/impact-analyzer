package contract

import (
	"fmt"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ComputeDiff computes the difference between old and new contracts
// Uses signature hash comparison to detect modifications
func ComputeDiff(oldContracts, newContracts []*model.Contract) (*model.ContractDiff, error) {
	// Dedupe inputs (multiple extractor paths may emit identical contracts)
	oldContracts = dedupeContracts(oldContracts)
	newContracts = dedupeContracts(newContracts)

	diff := &model.ContractDiff{
		ContractsAdded:    make([]*model.Contract, 0, len(newContracts)),
		ContractsRemoved:  make([]*model.Contract, 0, len(oldContracts)),
		ContractsModified: make([]*model.Contract, 0, min(len(oldContracts), len(newContracts))),
	}

	// Ensure all contracts have computed hashes
	for _, c := range oldContracts {
		if c.SignatureHash == "" {
			if err := c.ComputeSignatureHash(); err != nil {
				return nil, fmt.Errorf("failed to compute hash for old contract %s: %w", c.Identifier, err)
			}
		}
	}
	for _, c := range newContracts {
		if c.SignatureHash == "" {
			if err := c.ComputeSignatureHash(); err != nil {
				return nil, fmt.Errorf("failed to compute hash for new contract %s: %w", c.Identifier, err)
			}
		}
	}

	// Build hash maps for efficient lookup
	oldByHash := model.ContractsByHash(oldContracts)
	newByHash := model.ContractsByHash(newContracts)

	// Also build identifier maps for better matching
	oldByIdentifier := model.ContractsByIdentifier(oldContracts)
	newByIdentifier := model.ContractsByIdentifier(newContracts)

	// Track which contracts have been processed
	processedNew := make(map[string]bool, len(newContracts))

	// Find removed and modified contracts
	for _, oldContract := range oldContracts {
		oldHash := oldContract.SignatureHash

		// Check if hash exists in new contracts (exact match)
		if _, existsInNew := newByHash[oldHash]; existsInNew {
			// Same hash = unchanged, skip
			continue
		}

		// Hash not found - either removed or modified
		// Check if identifier exists in new contracts
		if newWithSameID, hasNewID := newByIdentifier[oldContract.Identifier]; hasNewID && len(newWithSameID) > 0 {
			// Identifier exists but hash differs -> Modified
			// Use the first match (usually there's only one)
			newContract := newWithSameID[0]
			diff.ContractsModified = append(diff.ContractsModified, newContract)
			processedNew[newContract.SignatureHash] = true
		} else {
			// Identifier not found -> Removed
			diff.ContractsRemoved = append(diff.ContractsRemoved, oldContract)
		}
	}

	// Find added contracts
	for _, newContract := range newContracts {
		newHash := newContract.SignatureHash

		// Skip if already processed as modified
		if processedNew[newHash] {
			continue
		}

		// Check if hash exists in old contracts
		if _, existsInOld := oldByHash[newHash]; existsInOld {
			// Same hash = unchanged, skip
			continue
		}

		// Check if identifier exists in old contracts
		if _, hasOldID := oldByIdentifier[newContract.Identifier]; !hasOldID {
			// Identifier not found -> Added
			diff.ContractsAdded = append(diff.ContractsAdded, newContract)
		}
		// If identifier exists but we got here, it was already processed as modified above
	}

	// Sort results for stable output
	model.SortContracts(diff.ContractsAdded)
	model.SortContracts(diff.ContractsRemoved)
	model.SortContracts(diff.ContractsModified)

	return diff, nil
}

// dedupeContracts removes duplicate contracts that emit from multiple extractor paths
// (e.g., SSA + AST fallback both producing the same contract).
// Uniqueness key: kind + identifier + file + line + signature_hash
func dedupeContracts(contracts []*model.Contract) []*model.Contract {
	if len(contracts) == 0 {
		return contracts
	}
	seen := make(map[string]bool, len(contracts))
	result := make([]*model.Contract, 0, len(contracts))
	for _, c := range contracts {
		// Compute hash if missing (best effort, ignore error here; ComputeDiff will retry)
		if c.SignatureHash == "" {
			_ = c.ComputeSignatureHash()
		}
		key := fmt.Sprintf("%s|%s|%s|%d|%s", c.Kind, c.Identifier, c.File, c.Line, c.SignatureHash)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, c)
	}
	return result
}
