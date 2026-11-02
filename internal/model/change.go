package model

// ChangeType represents the type of code change
type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeModified ChangeType = "modified"
	ChangeTypeDeleted  ChangeType = "deleted"
	ChangeTypeRenamed  ChangeType = "renamed"
)

// Change represents a code change detected in a diff
type Change struct {
	Type      ChangeType `json:"type"`
	Symbol    *Symbol    `json:"symbol"`
	OldSymbol *Symbol    `json:"old_symbol,omitempty"` // For renamed symbols
	File      string     `json:"file"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`

	// Detailed change information
	SignatureChanged bool `json:"signature_changed"`
	BodyChanged      bool `json:"body_changed"`
	CommentOnly      bool `json:"comment_only"`

	// BFS starting point metadata
	OutOfFunction bool   `json:"out_of_function,omitempty"`
	FallbackKind  string `json:"fallback_kind,omitempty"`
	NoiseFiltered bool   `json:"noise_filtered,omitempty"`
}

// IsBreaking returns true if the change is potentially breaking
func (c *Change) IsBreaking() bool {
	if c.Type == ChangeTypeDeleted {
		return c.Symbol.IsExported
	}
	if c.Type == ChangeTypeRenamed {
		return c.Symbol.IsExported || (c.OldSymbol != nil && c.OldSymbol.IsExported)
	}
	if c.Type == ChangeTypeModified {
		return c.SignatureChanged && c.Symbol.IsExported
	}
	return false
}
