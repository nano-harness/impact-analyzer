package contract

import (
	"runtime"
	"sync"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// extractBatchMu serializes whole extraction batches (BuildIndex, the git
// loader's working-tree loads). Every batch defers astutil.ClearParseCache,
// which closes tree-sitter trees a concurrently running batch could still be
// walking, so two batches must never interleave. Within a batch, extraction
// parallelizes freely across files: extractors are stateless, and the shared
// astutil parse cache is mutex-protected with read-only trees.
var extractBatchMu sync.Mutex

// mapFilesParallel runs fn over files with a worker pool bounded by
// GOMAXPROCS and returns the per-file contract slices concatenated in input
// order, so the merged result is byte-identical to running fn serially over
// the same list. fn must be safe for concurrent use across distinct files
// and must not mutate shared state other than the extractor/parse caches.
func mapFilesParallel(files []string, fn func(file string) []*model.Contract) []*model.Contract {
	results := make([][]*model.Contract, len(files))
	if len(files) == 0 {
		return nil
	}

	workers := runtime.GOMAXPROCS(0)
	if workers > len(files) {
		workers = len(files)
	}
	todo := make(chan int)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range todo {
				results[idx] = fn(files[idx])
			}
		}()
	}
	for i := range files {
		todo <- i
	}
	close(todo)
	wg.Wait()

	var total int
	for _, r := range results {
		total += len(r)
	}
	all := make([]*model.Contract, 0, total)
	for _, r := range results {
		all = append(all, r...)
	}
	return all
}
