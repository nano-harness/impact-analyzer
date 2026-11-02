package orchestrator

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// AnalysisCache provides result-level caching for impact analysis
// Cache entries are stored in <cacheDir>/.analysis-cache/<key>.json
type AnalysisCache struct {
	cacheDir string
	enabled  bool
	ttl      time.Duration // Cache entry TTL (0 = no expiration)
}

// NewAnalysisCache creates a new analysis cache
func NewAnalysisCache(cacheDir string, enabled bool) *AnalysisCache {
	return &AnalysisCache{
		cacheDir: filepath.Join(cacheDir, ".analysis-cache"),
		enabled:  enabled,
		ttl:      24 * time.Hour, // Default 24 hour TTL
	}
}

// CacheKeyParams carries the analysis-affecting parameters of a run that are
// not captured by the config file hash (CLI overrides bypass the config file,
// and some defaults never touch it). Every field changes the analysis output,
// so every field must change the cache key.
type CacheKeyParams struct {
	MaxDepth               int      // BFS traversal depth (--depth / max_depth)
	StopAtEntryPoints      bool     // stop_at_entry_points
	UseDeepAnalysis        bool     // go.use_deep_analysis (SSA/RTA edge pass)
	TraceAffectedContracts bool     // trace_affected_contracts (--no-trace-contracts)
	TraceMaxDepth          int      // trace_max_depth (--trace-depth)
	WithSurface            bool     // surface contract diff (--no-surface-diff)
	BuildTags              []string // go.build_tags (affects SSA edge set)
	ContentHash            string   // hash of the analyzed diff text (covers dirty worktrees)
}

// ComputeKey computes a cache key based on:
// - diffRef: git diff reference (resolved to commit SHA)
// - repoHEAD: current HEAD commit SHA
// - configHash: hash of the config file content
// - version: tool version
// - params: effective analysis parameters (depth, tracing, deep analysis, ...)
func (c *AnalysisCache) ComputeKey(diffRef, repoHEAD, configHash, version string, params CacheKeyParams) string {
	// Build composite key
	parts := []string{diffRef, repoHEAD, configHash, version,
		fmt.Sprintf("depth=%d", params.MaxDepth),
		fmt.Sprintf("deep=%t", params.UseDeepAnalysis),
		fmt.Sprintf("trace=%t:%d", params.TraceAffectedContracts, params.TraceMaxDepth),
		fmt.Sprintf("surface=%t", params.WithSurface),
	}
	if params.StopAtEntryPoints {
		parts = append(parts, "stop-at-entry")
	}
	if len(params.BuildTags) > 0 {
		parts = append(parts, "tags="+strings.Join(params.BuildTags, ","))
	}
	if params.ContentHash != "" {
		parts = append(parts, "content="+params.ContentHash)
	}

	// Hash the parts
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte("|"))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// Get retrieves a cached result by key
// Returns nil, false if not found or expired
func (c *AnalysisCache) Get(key string) (*model.ImpactResult, bool) {
	if !c.enabled {
		return nil, false
	}

	cachePath := filepath.Join(c.cacheDir, key+".json")

	// Check if file exists
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, false
	}

	// Check TTL
	if c.ttl > 0 && time.Since(info.ModTime()) > c.ttl {
		// Expired, remove it
		_ = os.Remove(cachePath)
		return nil, false
	}

	// Read and unmarshal
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false
	}

	var result model.ImpactResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, false
	}

	return &result, true
}

// Put stores a result in the cache
func (c *AnalysisCache) Put(key string, result *model.ImpactResult) error {
	if !c.enabled {
		return nil
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cachePath := filepath.Join(c.cacheDir, key+".json")

	// Marshal result
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Write atomically via temp file
	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	if err := os.Rename(tmpPath, cachePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	return nil
}

// Clear removes all cache entries
func (c *AnalysisCache) Clear() error {
	if !c.enabled {
		return nil
	}

	return os.RemoveAll(c.cacheDir)
}

// Evict removes expired cache entries
func (c *AnalysisCache) Evict() error {
	if !c.enabled || c.ttl == 0 {
		return nil
	}

	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(c.cacheDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > c.ttl {
			_ = os.Remove(path)
		}
	}

	return nil
}

// ComputeConfigHash computes a hash of the config file content
func ComputeConfigHash(configPath string) (string, error) {
	if configPath == "" || !fileExists(configPath) {
		return "no-config", nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
