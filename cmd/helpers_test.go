package cmd

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/spf13/cobra"
)

func depthCmd(t *testing.T, setFlag bool) *cobra.Command {
	t.Helper()
	c := &cobra.Command{}
	var depth int
	c.Flags().IntVar(&depth, "depth", 5, "")
	if setFlag {
		if err := c.Flags().Set("depth", "5"); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func TestEffectiveDepth(t *testing.T) {
	cfg := &config.Config{}
	cfg.Analysis.MaxDepth = 2

	if got := effectiveDepth(depthCmd(t, true), 5, cfg); got != 5 {
		t.Fatalf("explicit --depth should win over config: got %d, want 5", got)
	}
	if got := effectiveDepth(depthCmd(t, false), 5, cfg); got != 2 {
		t.Fatalf("config analysis.max_depth should apply when flag unset: got %d, want 2", got)
	}

	empty := &config.Config{}
	if got := effectiveDepth(depthCmd(t, false), 5, empty); got != 5 {
		t.Fatalf("zero config max_depth should fall back to flag default: got %d, want 5", got)
	}
	if got := effectiveDepth(depthCmd(t, false), 5, nil); got != 5 {
		t.Fatalf("nil config should fall back to flag default: got %d, want 5", got)
	}
	if got := effectiveDepth(nil, 5, cfg); got != 2 {
		t.Fatalf("nil command should behave as flag unset: got %d, want 2", got)
	}
}
