package config

import (
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	// Simplified top-level fields (parent requirement format - new)
	Language string          `mapstructure:"language"` // Main language: go/java/javascript/typescript
	Exclude  []string        `mapstructure:"exclude"`  // Top-level exclude (merged to Analysis.ExcludePaths)
	Analyze  AnalyzeShortcut `mapstructure:"analyze"`  // Top-level analyze section (merged to Analysis.MaxDepth)

	// Detailed language-specific settings (kept for backward compatibility)
	Go         GoConfig         `mapstructure:"go"`
	Java       JavaConfig       `mapstructure:"java"`
	JavaScript JavaScriptConfig `mapstructure:"javascript"`
	TypeScript TypeScriptConfig `mapstructure:"typescript"`
	Python     PythonConfig     `mapstructure:"python"`

	// Index settings
	Index IndexConfig `mapstructure:"index"`

	// Analysis settings
	Analysis AnalysisConfig `mapstructure:"analysis"`

	// Call graph settings
	Callgraph CallgraphConfig `mapstructure:"callgraph"`

	// Risk classification rules
	Risk RiskConfig `mapstructure:"risk"`

	// Orphan detection settings
	Orphan OrphanConfig `mapstructure:"orphan"`
}

// AnalyzeShortcut contains simplified analyze section configuration
type AnalyzeShortcut struct {
	Depth             int  `mapstructure:"depth"`
	StopAtEntryPoints bool `mapstructure:"stop_at_entry_points"`
}

// GoConfig contains Go-specific configuration
type GoConfig struct {
	UseDeepAnalysis   bool     `mapstructure:"use_deep_analysis"` // Enable go/types + callgraph
	ReflectionAware   bool     `mapstructure:"reflection_aware"`  // Reflection-conservative model
	TestPatterns      []string `mapstructure:"test_patterns"`
	BuildTags         []string `mapstructure:"build_tags"`
	CustomEntryPoints []string `mapstructure:"custom_entry_points"` // #21: Custom RTA entry points (qualified names)
}

// JavaConfig contains Java-specific configuration
type JavaConfig struct {
	TestPatterns []string `mapstructure:"test_patterns"`
	SourceDirs   []string `mapstructure:"source_dirs"`
}

// JavaScriptConfig contains JavaScript-specific configuration
type JavaScriptConfig struct {
	TestPatterns []string `mapstructure:"test_patterns"`
}

// TypeScriptConfig contains TypeScript-specific configuration
type TypeScriptConfig struct {
	TestPatterns []string `mapstructure:"test_patterns"`
	ConfigFile   string   `mapstructure:"config_file"` // tsconfig.json path
}

// PythonConfig contains Python-specific configuration
type PythonConfig struct {
	TestPatterns []string `mapstructure:"test_patterns"`
}

// IndexConfig contains indexing configuration
type IndexConfig struct {
	CacheDir        string   `mapstructure:"cache_dir"`
	IgnorePatterns  []string `mapstructure:"ignore_patterns"`
	IncrementalMode bool     `mapstructure:"incremental_mode"`
}

// AnalysisConfig contains analysis configuration
type AnalysisConfig struct {
	MaxDepth               int                 `mapstructure:"max_depth"`
	IncludeTests           bool                `mapstructure:"include_tests"`
	IncludeVendor          bool                `mapstructure:"include_vendor"`
	TrackUnresolved        bool                `mapstructure:"track_unresolved"`         // Track unresolved calls in JS/TS
	ExcludePaths           []string            `mapstructure:"exclude_paths"`            // Paths to exclude (detailed field)
	StopAtEntryPoints      bool                `mapstructure:"stop_at_entry_points"`     // Stop call graph traversal at entry points
	EntryPointPatterns     []EntryPointPattern `mapstructure:"entry_point_patterns"`     // Entry point patterns for stopping traversal
	TraceAffectedContracts bool                `mapstructure:"trace_affected_contracts"` // Enable contract tracing (method -> contracts)
	TraceMaxDepth          int                 `mapstructure:"trace_max_depth"`          // Max depth for contract tracing
	ImportExpansionDepth   int                 `mapstructure:"import_expansion_depth"`   // Reverse import BFS expansion depth (default 2)
}

// EntryPointPattern defines a pattern for identifying entry points
// Entry points are boundaries where call graph traversal should stop
type EntryPointPattern struct {
	Kind          string `mapstructure:"kind,omitempty"`           // Contract kind (cli_command, http_route, etc.) - uses contract extractor
	FilePattern   string `mapstructure:"file_pattern,omitempty"`   // Glob pattern for files (fallback)
	SymbolPattern string `mapstructure:"symbol_pattern,omitempty"` // Regex pattern for symbol names (fallback)
}

// CallgraphConfig contains call graph resolution settings
type CallgraphConfig struct {
	PermissiveFallback bool `mapstructure:"permissive_fallback"` // #15: Allow bare-name fallback across owners
}

// RiskConfig contains risk classification rules
type RiskConfig struct {
	// Thresholds for risk scoring
	BreakingThreshold float64 `mapstructure:"breaking_threshold"` // Default: 75
	WarningThreshold  float64 `mapstructure:"warning_threshold"`  // Default: 30

	// Weight factors for composite scoring
	CallerWeight    float64 `mapstructure:"caller_weight"`    // Default: 0.4
	ExportWeight    float64 `mapstructure:"export_weight"`    // Default: 0.3
	InterfaceWeight float64 `mapstructure:"interface_weight"` // Default: 0.3
}

// OrphanConfig contains orphan-code detection settings.
type OrphanConfig struct {
	ExtraEntries       []interface{}     `mapstructure:"extra_entries"`
	Heuristics         OrphanHeuristics  `mapstructure:"heuristics"`
	Ignore             []string          `mapstructure:"ignore"`
	TypeRefEdges       bool              `mapstructure:"type_ref_edges"`
	HighGateMinCallers int               `mapstructure:"high_gate_min_callers"`
	Confidence         OrphanConfidence  `mapstructure:"confidence"`
	NoiseFilter        OrphanNoiseFilter `mapstructure:"noise_filter"`
}

// OrphanHeuristics controls fallback entry-point detection.
type OrphanHeuristics struct {
	EnableFilename   bool `mapstructure:"enable_filename"`
	EnableInit       bool `mapstructure:"enable_init"`
	EnableReflection bool `mapstructure:"enable_reflection"`
	EnableTestEntry  bool `mapstructure:"enable_test_entry"`
}

// OrphanConfidence controls confidence scoring weights.
type OrphanConfidence struct {
	NewCodeWeight             float64 `mapstructure:"new_code_weight"`
	FileIsolatedWeight        float64 `mapstructure:"file_isolated_weight"`
	HasOldVersionWeight       float64 `mapstructure:"has_old_version_weight"`
	ExportedOffset            float64 `mapstructure:"exported_offset"`
	PythonDynamicOffset       float64 `mapstructure:"python_dynamic_offset"`
	KindFunctionWeight        float64 `mapstructure:"kind_function_weight"`
	KindVariableOffset        float64 `mapstructure:"kind_variable_offset"`
	KindUnexportedConstOffset float64 `mapstructure:"kind_unexported_const_offset"`
	UnexportedOffset          float64 `mapstructure:"unexported_offset"`
	LargeBodyWeight           float64 `mapstructure:"large_body_weight"`
	OnlyTestCallersOffset     float64 `mapstructure:"only_test_callers_offset"`
	LocalVariablePenalty      float64 `mapstructure:"local_variable_penalty"`
	UnexportedConstPenalty    float64 `mapstructure:"unexported_const_penalty"`
	TinyStructPenalty         float64 `mapstructure:"tiny_struct_penalty"`
	ShortFunctionPenalty      float64 `mapstructure:"short_function_penalty"`
	WeakReachOffset           float64 `mapstructure:"weak_reach_offset"`
	SSAConfirmedWeight        float64 `mapstructure:"ssa_confirmed_weight"`
}

// OrphanNoiseFilter controls default low-signal candidate filtering.
type OrphanNoiseFilter struct {
	SkipLocalVariables      bool `mapstructure:"skip_local_variables"`
	SkipUnexportedConstants bool `mapstructure:"skip_unexported_constants"`
	SkipTinyStructs         bool `mapstructure:"skip_tiny_structs"`
	SkipShortFunctions      bool `mapstructure:"skip_short_functions"`
}

// LoadConfig loads configuration from file and environment
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	// Set defaults
	setDefaults()

	// Unmarshal into struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	// Normalize config to merge simplified and detailed formats
	normalizeConfig(cfg)

	return cfg, nil
}

func setDefaults() {
	// Go defaults
	viper.SetDefault("go.use_deep_analysis", true) // Enable SSA/RTA for orphan detection by default
	viper.SetDefault("go.reflection_aware", true)  // Reflection-conservative model by default
	viper.SetDefault("go.test_patterns", []string{"*_test.go"})

	// Java defaults
	viper.SetDefault("java.test_patterns", []string{"*Test.java", "*Tests.java"})
	viper.SetDefault("java.source_dirs", []string{"src/main/java", "src/test/java"})

	// JavaScript defaults
	viper.SetDefault("javascript.test_patterns", []string{"*.test.js", "*.spec.js", "**/__tests__/**/*.js"})

	// TypeScript defaults
	viper.SetDefault("typescript.test_patterns", []string{"*.test.ts", "*.spec.ts", "**/__tests__/**/*.ts"})

	// Python defaults
	viper.SetDefault("python.test_patterns", []string{"test_*.py", "*_test.py", "tests/**/*.py", "conftest.py"})

	// Index defaults
	viper.SetDefault("index.cache_dir", ".impact/cache")
	viper.SetDefault("index.ignore_patterns", []string{"node_modules", "vendor", ".git", "dist", "build", "target"})
	viper.SetDefault("index.incremental_mode", true)

	// Analysis defaults
	viper.SetDefault("analysis.max_depth", -1)
	viper.SetDefault("analysis.include_tests", true)
	viper.SetDefault("analysis.include_vendor", false)
	viper.SetDefault("analysis.track_unresolved", true)
	viper.SetDefault("analysis.trace_affected_contracts", true)
	viper.SetDefault("analysis.trace_max_depth", 8)
	viper.SetDefault("analysis.import_expansion_depth", 2)

	// Callgraph defaults
	viper.SetDefault("callgraph.permissive_fallback", true) // #15: Default true for transition period

	// Risk defaults
	viper.SetDefault("risk.breaking_threshold", 75.0)
	viper.SetDefault("risk.warning_threshold", 30.0)
	viper.SetDefault("risk.caller_weight", 0.4)
	viper.SetDefault("risk.export_weight", 0.3)
	viper.SetDefault("risk.interface_weight", 0.3)

	// Orphan defaults
	viper.SetDefault("orphan.type_ref_edges", true)
	viper.SetDefault("orphan.high_gate_min_callers", 5)
	viper.SetDefault("orphan.heuristics.enable_filename", true)
	viper.SetDefault("orphan.heuristics.enable_init", true)
	viper.SetDefault("orphan.heuristics.enable_reflection", true)
	viper.SetDefault("orphan.heuristics.enable_test_entry", false)
	viper.SetDefault("orphan.confidence.new_code_weight", 3.0)
	viper.SetDefault("orphan.confidence.file_isolated_weight", 2.0)
	viper.SetDefault("orphan.confidence.has_old_version_weight", 2.0)
	viper.SetDefault("orphan.confidence.exported_offset", -1.5)
	viper.SetDefault("orphan.confidence.python_dynamic_offset", -1.0)
	viper.SetDefault("orphan.confidence.kind_function_weight", 1.0)
	viper.SetDefault("orphan.confidence.kind_variable_offset", -1.5)
	viper.SetDefault("orphan.confidence.kind_unexported_const_offset", -1.0)
	viper.SetDefault("orphan.confidence.unexported_offset", -1.0)
	viper.SetDefault("orphan.confidence.large_body_weight", 0.5)
	viper.SetDefault("orphan.confidence.only_test_callers_offset", -1.0)
	viper.SetDefault("orphan.confidence.local_variable_penalty", -0.8)
	viper.SetDefault("orphan.confidence.unexported_const_penalty", -0.6)
	viper.SetDefault("orphan.confidence.tiny_struct_penalty", -0.5)
	viper.SetDefault("orphan.confidence.short_function_penalty", -0.4)
	viper.SetDefault("orphan.confidence.weak_reach_offset", -2.0)
	viper.SetDefault("orphan.confidence.ssa_confirmed_weight", 2.0)
	viper.SetDefault("orphan.noise_filter.skip_local_variables", true)
	viper.SetDefault("orphan.noise_filter.skip_unexported_constants", true)
	viper.SetDefault("orphan.noise_filter.skip_tiny_structs", true)
	viper.SetDefault("orphan.noise_filter.skip_short_functions", true)
}

// normalizeConfig merges simplified format fields into detailed fields
// Simplified format (cfg.Analyze.Depth) takes precedence over detailed format (cfg.Analysis.MaxDepth)
// Top-level cfg.Exclude is merged with cfg.Analysis.ExcludePaths
func normalizeConfig(cfg *Config) {
	// Merge analyze.depth into analysis.max_depth if specified
	if cfg.Analyze.Depth != 0 {
		cfg.Analysis.MaxDepth = cfg.Analyze.Depth
	}

	// Merge analyze.stop_at_entry_points into analysis.stop_at_entry_points if specified
	if cfg.Analyze.StopAtEntryPoints {
		cfg.Analysis.StopAtEntryPoints = true
	}

	// Merge top-level exclude into analysis.exclude_paths
	if len(cfg.Exclude) > 0 {
		cfg.Analysis.ExcludePaths = mergeUnique(cfg.Analysis.ExcludePaths, cfg.Exclude)
	}

	// Ensure default exclude paths are present
	defaults := DefaultExcludePaths()
	cfg.Analysis.ExcludePaths = mergeUnique(cfg.Analysis.ExcludePaths, defaults)
}

// DefaultExcludePaths returns the default paths to exclude
func DefaultExcludePaths() []string {
	return []string{
		"vendor/**",
		"node_modules/**",
		".git/**",
		"dist/**",
		"build/**",
		"target/**",
		".impact/**",
	}
}

// MergeExcludePaths merges exclude paths from three sources with priority: CLI > config > defaults
func MergeExcludePaths(cliExcludes []string, cfg *Config) []string {
	var result []string

	// Start with CLI excludes (highest priority)
	result = append(result, cliExcludes...)

	// Add config excludes (both top-level and detailed)
	if cfg != nil {
		result = append(result, cfg.Exclude...)
		result = append(result, cfg.Analysis.ExcludePaths...)
	}

	// Add defaults (lowest priority)
	result = append(result, DefaultExcludePaths()...)

	// Remove duplicates
	return uniqueStrings(result)
}

// mergeUnique merges two slices and removes duplicates
func mergeUnique(slice1, slice2 []string) []string {
	result := append([]string{}, slice1...)
	result = append(result, slice2...)
	return uniqueStrings(result)
}

// uniqueStrings removes duplicates from a string slice while preserving order
func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if s == "" {
			continue
		}
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
