package orchestrator

import (
	"fmt"
	"os"

	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// contractIndexCache caches contract index to avoid rebuilding
type contractIndexCache struct {
	index *contract.ContractIndex
	// Could add invalidation logic based on repo state if needed
}

var globalContractIndexCache *contractIndexCache

// enrichWithAffectedContracts traces each changed method upward to find affected contracts
func (o *Orchestrator) enrichWithAffectedContracts(
	result *model.ImpactResult,
	maxDepth int,
) error {
	verbose := os.Getenv("IMPACT_VERBOSE") != ""

	// Build contract index (with caching)
	var contractIdx *contract.ContractIndex
	if globalContractIndexCache != nil && globalContractIndexCache.index != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "[ContractEnricher] Using cached contract index\n")
		}
		contractIdx = globalContractIndexCache.index
	} else {
		if verbose {
			fmt.Fprintf(os.Stderr, "[ContractEnricher] Building contract index...\n")
		}
		var err error
		contractIdx, err = contract.BuildIndex(o.repoPath, o.excludePaths)
		if err != nil {
			return fmt.Errorf("failed to build contract index: %w", err)
		}
		// Cache for subsequent calls
		globalContractIndexCache = &contractIndexCache{index: contractIdx}
		if verbose {
			fmt.Fprintf(os.Stderr, "[ContractEnricher] Index built: %d contracts, %d handlers\n",
				contractIdx.Count(), contractIdx.HandlerCount())
		}
	}

	// If no handlers indexed, nothing to do
	if contractIdx.HandlerCount() == 0 {
		if verbose {
			fmt.Fprintf(os.Stderr, "[ContractEnricher] No handlers in index, skipping enrichment\n")
		}
		return nil
	}

	// Enrich each change with affected contracts
	totalAffectedContracts := 0
	for _, changeImpact := range result.Changes {
		if changeImpact.Change == nil || changeImpact.Change.Symbol == nil {
			continue
		}

		sym := changeImpact.Change.Symbol

		// Traverse upward from this symbol, stopping at handlers, collecting paths
		traversalResult := o.callGraph.TraverseCallers(sym, graph.TraversalOptions{
			MaxDepth:     maxDepth,
			IsStop:       contractIdx.IsHandler,
			CollectPaths: true,
		})

		// For each path that ends at a handler, look up the contracts
		for i, path := range traversalResult.Paths {
			if i >= len(traversalResult.Callers) {
				break
			}
			handler := traversalResult.Callers[i]

			// Look up contracts for this handler
			contracts := contractIdx.LookupByHandler(handler)
			for _, c := range contracts {
				affectedContract := &model.AffectedContract{
					Contract:  c,
					CallPath:  path.Chain,
					Depth:     path.Depth,
					RiskLevel: changeImpact.RiskLevel,
				}
				changeImpact.AffectedContracts = append(changeImpact.AffectedContracts, affectedContract)
				totalAffectedContracts++
			}
		}

		if verbose && len(changeImpact.AffectedContracts) > 0 {
			fmt.Fprintf(os.Stderr, "[ContractEnricher] %s affects %d contract(s)\n",
				sym.QualifiedName(), len(changeImpact.AffectedContracts))
		}
	}

	result.AffectedContractsCount = totalAffectedContracts

	if verbose {
		fmt.Fprintf(os.Stderr, "[ContractEnricher] Total affected contracts: %d\n", totalAffectedContracts)
	}

	return nil
}
