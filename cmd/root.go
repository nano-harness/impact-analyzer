package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile   string
	verbose   bool
	format    string
	lang      string
	maxTokens int
	version   string // Injected version
	indexDir  string // External index directory
)

var rootCmd = &cobra.Command{
	Use:   "impact",
	Short: "Impact - Multi-language code change impact analyzer",
	Long: `Impact is a code change impact analysis CLI tool that helps you understand
the blast radius of your code changes across Go, Java, JavaScript, TypeScript, Python, and Rust projects.

It analyzes call graphs, dependency chains, and test mappings to identify:
- Which functions/methods are affected by your changes
- Breaking changes vs safe refactorings
- Which tests need to be run
- Risk levels for each change`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Unify verbosity: --verbose (or verbose in config/env) also activates
		// internal diagnostics gated on the IMPACT_VERBOSE env var.
		if !verbose && viper.GetBool("verbose") {
			verbose = true
		}
		if verbose {
			os.Setenv("IMPACT_VERBOSE", "1")
		}
	},
}

// SetVersion sets the version string injected at build time.
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

// GetVersion returns the current version string
func GetVersion() string {
	if version == "" {
		return "dev"
	}
	return version
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .impact.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "text", "output format (text, json, markdown, mermaid, llm-context)")
	rootCmd.PersistentFlags().StringVarP(&lang, "lang", "l", "", "target language (go, java, javascript, typescript, python, rust) - auto-detected if not specified")
	rootCmd.PersistentFlags().IntVar(&maxTokens, "max-tokens", 8000, "maximum tokens for llm-context format (0 = unlimited)")
	rootCmd.PersistentFlags().StringVar(&indexDir, "index-dir", "", "external index/cache directory (overrides INDEX_CACHE_DIR env and index.cache_dir config; default: .impact/cache)")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))
	viper.BindPFlag("lang", rootCmd.PersistentFlags().Lookup("lang"))
	viper.BindPFlag("max_tokens", rootCmd.PersistentFlags().Lookup("max-tokens"))
	viper.BindPFlag("index.cache_dir", rootCmd.PersistentFlags().Lookup("index-dir"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for .impact.yaml in current directory
		viper.AddConfigPath(".")
		viper.SetConfigName(".impact")
		viper.SetConfigType("yaml")
	}

	// Map config keys to env vars with dots replaced by underscores,
	// e.g. index.cache_dir -> INDEX_CACHE_DIR, max_tokens -> MAX_TOKENS.
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			// No config file present; defaults are fine.
			return
		}
		name := cfgFile
		if name == "" {
			name = viper.ConfigFileUsed()
		}
		if name == "" {
			name = ".impact.yaml"
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to load config file %s: %v\n", name, err)
		fmt.Fprintln(os.Stderr, "Continuing with default configuration.")
	} else if verbose {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
