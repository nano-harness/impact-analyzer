package orchestrator

import (
	"fmt"
	"os"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/contract"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// startSurfaceContractLoad kicks off surface-diff contract loading (git
// worktree checkout at the base ref plus contract extraction on both sides)
// on a background goroutine. It is a no-op when there is nothing to load.
// The goroutine only sends its result into a buffered channel — it never
// touches orchestrator state — so an abandoned load (run error path) is
// leak- and race-free. Contract extraction batches are serialized against a
// concurrently running contract.BuildIndex by the contract package's batch
// mutex, preserving the astutil parse-cache batch invariant.
func (o *Orchestrator) startSurfaceContractLoad(diffRef string, changedFiles []string) {
	if len(changedFiles) == 0 || len(contract.All()) == 0 {
		return
	}
	baseRef := o.parseBaseRef(diffRef)
	repoPath := o.repoPath
	ch := make(chan surfaceLoadResult, 1)
	o.surfaceCh = ch
	go func() {
		oldContracts, newContracts, err := contract.LoadContractsForDiff(repoPath, baseRef, changedFiles, contract.All())
		ch <- surfaceLoadResult{oldContracts: oldContracts, newContracts: newContracts, err: err}
	}()
}

// addSurfaceDiff computes and adds surface diff to the result
func (o *Orchestrator) addSurfaceDiff(result *model.ImpactResult, diffRef string, changedFiles []string) {
	// This is a best-effort operation - don't fail the entire analysis if it fails

	// Get all registered contract extractors
	extractors := contract.All()
	if len(extractors) == 0 {
		fmt.Fprintf(os.Stderr, "[surface] skip: no extractors registered\n")
		return // No extractors available
	}

	// Parse base ref from diffRef
	baseRef := o.parseBaseRef(diffRef)
	fmt.Fprintf(os.Stderr, "[surface] loading contracts: baseRef=%s, files=%d, extractors=%d\n",
		baseRef, len(changedFiles), len(extractors))

	// Load contracts from old and new versions, joining the background load
	// started at the beginning of the run when present.
	var oldContracts, newContracts []*model.Contract
	var err error
	if o.surfaceCh != nil {
		res := <-o.surfaceCh
		o.surfaceCh = nil
		oldContracts, newContracts, err = res.oldContracts, res.newContracts, res.err
	} else {
		oldContracts, newContracts, err = contract.LoadContractsForDiff(o.repoPath, baseRef, changedFiles, extractors)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[surface] LoadContractsForDiff failed: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[surface] loaded: old=%d, new=%d\n", len(oldContracts), len(newContracts))

	// Compute diff
	contractDiff, err := contract.ComputeDiff(oldContracts, newContracts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[surface] ComputeDiff failed: %v\n", err)
		return
	}

	// Only add if there are actual changes
	if contractDiff.IsEmpty() {
		fmt.Fprintf(os.Stderr, "[surface] diff is empty (no changes detected)\n")
		return
	}

	fmt.Fprintf(os.Stderr, "[surface] diff non-empty: added=%d, removed=%d, modified=%d -> set on result\n",
		len(contractDiff.ContractsAdded), len(contractDiff.ContractsRemoved), len(contractDiff.ContractsModified))
	result.SurfaceDiff = contractDiff

	// Link contracts to causing changes (reverse direction)
	o.linkContractsToChanges(result)
}

// parseBaseRef extracts the base ref from a diff specification
func (o *Orchestrator) parseBaseRef(diffSpec string) string {
	if strings.Contains(diffSpec, "...") {
		parts := strings.Split(diffSpec, "...")
		return parts[0]
	}
	if strings.Contains(diffSpec, "..") {
		parts := strings.Split(diffSpec, "..")
		return parts[0]
	}
	return diffSpec
}

// linkContractsToChanges performs reverse linking: for each contract in SurfaceDiff,
// populate CausingChanges based on the forward AffectedContracts in each ChangeImpact
func (o *Orchestrator) linkContractsToChanges(result *model.ImpactResult) {
	if result.SurfaceDiff == nil || len(result.Changes) == 0 {
		return
	}

	verbose := os.Getenv("IMPACT_VERBOSE") != ""

	// Build a map: contract identifier -> causing symbols
	contractToCauses := make(map[string][]*model.Symbol)

	for _, changeImpact := range result.Changes {
		if changeImpact.Change == nil || changeImpact.Change.Symbol == nil {
			continue
		}
		changeSym := changeImpact.Change.Symbol

		for _, affectedContract := range changeImpact.AffectedContracts {
			if affectedContract.Contract == nil {
				continue
			}
			contractID := affectedContract.Contract.Identifier
			contractToCauses[contractID] = append(contractToCauses[contractID], changeSym)
		}
	}

	// Apply to Modified and Removed contracts in SurfaceDiff
	for _, contract := range result.SurfaceDiff.ContractsModified {
		if causes, ok := contractToCauses[contract.Identifier]; ok {
			contract.CausingChanges = causes
			if verbose {
				fmt.Fprintf(os.Stderr, "[surface] Contract %s caused by %d change(s)\n",
					contract.Identifier, len(causes))
			}
		}
	}

	for _, contract := range result.SurfaceDiff.ContractsRemoved {
		if causes, ok := contractToCauses[contract.Identifier]; ok {
			contract.CausingChanges = causes
			if verbose {
				fmt.Fprintf(os.Stderr, "[surface] Contract %s caused by %d change(s)\n",
					contract.Identifier, len(causes))
			}
		}
	}
}
