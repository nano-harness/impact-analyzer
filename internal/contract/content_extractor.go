package contract

import (
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// ContentExtractor is an optional extension of ContractExtractor for extractors
// that can operate on caller-provided file content. Index building reads each
// repository file once and dispatches through this interface when the extractor
// supports it, instead of letting every extractor re-read the file from disk
// in CanHandle/Extract. Extractors that do not implement it keep the original
// read-per-call behavior via the fallback in the helpers below.
type ContentExtractor interface {
	ContractExtractor
	// CanHandleContent is equivalent to CanHandle but uses pre-read content.
	// content must come from a successful read of filePath.
	CanHandleContent(filePath string, content []byte) bool
	// ExtractContent is equivalent to Extract but uses pre-read content.
	ExtractContent(filePath string, content []byte) ([]*model.Contract, error)
}

// canHandleContent reports whether ex handles filePath, preferring the
// content-based path when the extractor implements ContentExtractor.
// content must come from a successful read of filePath; on read failure
// callers should use ex.CanHandle directly to preserve legacy behavior.
func canHandleContent(ex ContractExtractor, filePath string, content []byte) bool {
	if ce, ok := ex.(ContentExtractor); ok {
		return ce.CanHandleContent(filePath, content)
	}
	return ex.CanHandle(filePath)
}

// extractContent extracts contracts from filePath, preferring the
// content-based path when the extractor implements ContentExtractor.
// Same content contract as canHandleContent.
func extractContent(ex ContractExtractor, filePath string, content []byte) ([]*model.Contract, error) {
	if ce, ok := ex.(ContentExtractor); ok {
		return ce.ExtractContent(filePath, content)
	}
	return ex.Extract(filePath)
}
