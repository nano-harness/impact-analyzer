package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nano-harness/impact-analyzer/internal/config"
	langpkg "github.com/nano-harness/impact-analyzer/internal/lang"
	"github.com/nano-harness/impact-analyzer/internal/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostic checks on impact setup",
	Long: `Runs a series of diagnostic checks to ensure impact is properly configured
and can analyze your repository. Useful for troubleshooting issues.

Examples:
  # Run all diagnostic checks
  impact doctor

  # Run with verbose output
  impact doctor --verbose`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println("Impact Doctor")
	fmt.Println("=============")
	fmt.Println()

	checkCount := 0
	passCount := 0
	repoLangs := detectRepoLanguages()

	// Check 1: Language adapters
	checkCount++
	fmt.Printf("[%d/7] Checking language adapters... ", checkCount)
	adapters := langpkg.Available()
	if len(adapters) == 0 {
		fmt.Println("✗ FAIL")
		fmt.Println("  No language adapters registered. This is a critical bug.")
		fmt.Println("  Please reinstall impact or report this issue.")
	} else {
		fmt.Println("✓ OK")
		fmt.Print("  Registered: ")
		for i, lang := range adapters {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(lang)
		}
		fmt.Println()
		passCount++
	}

	// Check 2: Git availability
	checkCount++
	fmt.Printf("[%d/7] Checking git... ", checkCount)
	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println("✗ FAIL")
		fmt.Println("  git not found in PATH")
		fmt.Println("  Install git or add it to your PATH")
	} else {
		gitVersionCmd := exec.Command("git", "--version")
		if output, err := gitVersionCmd.Output(); err == nil {
			fmt.Println("✓ OK")
			fmt.Printf("  %s", string(output))
			passCount++
		} else {
			fmt.Println("✗ FAIL")
			fmt.Println("  git found but failed to run")
		}
	}

	// Check 3: Current repository
	checkCount++
	fmt.Printf("[%d/7] Checking current directory... ", checkCount)
	if _, err := os.Stat(".git"); err == nil {
		fmt.Println("✓ OK")
		fmt.Println("  Git repository detected")

		// Try to detect language
		if len(repoLangs) > 0 {
			primary := repoLangs[0]
			fmt.Printf("  Primary language: %s\n", primary.name)
			if langpkg.IsRegistered(primary.lang) {
				fmt.Printf("  %s adapter: available\n", primary.name)
			}
		}

		passCount++
	} else {
		fmt.Println("⚠ WARNING")
		fmt.Println("  Not a git repository")
		fmt.Println("  Some commands require a git repository")
	}

	// Check 4: Go toolchain (required for SSA deep analysis)
	checkCount++
	fmt.Printf("[%d/7] Checking Go toolchain... ", checkCount)
	doctorCfg, cfgErr := config.LoadConfig() // best effort; reused by cache check below
	if cfgErr == nil && doctorCfg != nil && doctorCfg.Go.UseDeepAnalysis {
		if goPath, err := exec.LookPath("go"); err != nil {
			fmt.Println("⚠ WARNING")
			fmt.Println("  go.use_deep_analysis is enabled but `go` was not found in PATH")
			fmt.Println("  SSA deep analysis will silently degrade")
			fmt.Println("  Install Go or set go.use_deep_analysis: false in .impact.yaml")
		} else {
			fmt.Println("✓ OK")
			fmt.Printf("  go found: %s\n", goPath)
			passCount++
		}
	} else {
		fmt.Println("✓ OK")
		fmt.Println("  go.use_deep_analysis disabled; Go toolchain not required")
		passCount++
	}

	// Check 5: Supported source files present
	checkCount++
	fmt.Printf("[%d/7] Checking for supported source files... ", checkCount)
	if len(repoLangs) > 0 {
		fmt.Println("✓ OK")
		fmt.Print("  Found: ")
		for i, probe := range repoLangs {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(probe.name)
		}
		fmt.Println()
		passCount++
	} else {
		fmt.Println("⚠ WARNING")
		fmt.Println("  No supported source files found (searched repo root and two levels deep)")
		fmt.Println("  Supported extensions: .go, .java, .ts, .js, .rs, .py")
	}

	// Check 6: Configuration file
	checkCount++
	fmt.Printf("[%d/7] Checking configuration... ", checkCount)
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		fmt.Println("✓ OK")
		fmt.Printf("  Config file: %s\n", configFile)
		passCount++
	} else {
		fmt.Println("⚠ INFO")
		fmt.Println("  No .impact.yaml found, using defaults")
		fmt.Println("  This is normal for first-time use")
		passCount++ // Not a failure
	}

	// Check 7: Cache directory
	checkCount++
	fmt.Printf("[%d/7] Checking cache directory... ", checkCount)
	cacheDir := resolveIndexDir(doctorCfg)
	if info, err := os.Stat(cacheDir); err == nil {
		if info.IsDir() {
			fmt.Println("✓ OK")
			fmt.Printf("  Cache directory: %s\n", cacheDir)

			// Count files in cache
			if entries, err := os.ReadDir(cacheDir); err == nil {
				fmt.Printf("  Cached files: %d\n", len(entries))
			}
			passCount++
		} else {
			fmt.Println("✗ FAIL")
			fmt.Printf("  %s exists but is not a directory\n", cacheDir)
		}
	} else {
		fmt.Println("⚠ INFO")
		fmt.Printf("  Cache directory %s not yet created\n", cacheDir)
		fmt.Println("  Will be created on first analysis")
		passCount++ // Not a failure
	}

	fmt.Println()
	fmt.Println("===============")
	fmt.Printf("Summary: %d/%d checks passed\n", passCount, checkCount)
	fmt.Println()

	if passCount < checkCount-1 { // Allow 1 failure
		fmt.Println("⚠ Some checks failed. Please address the issues above.")
		return fmt.Errorf("%d check(s) failed", checkCount-passCount)
	}

	fmt.Println("✓ Impact is properly configured!")
	return nil
}

type langProbe struct {
	ext  string
	lang model.Language
	name string
}

var langProbes = []langProbe{
	{".go", model.LanguageGo, "Go"},
	{".java", model.LanguageJava, "Java"},
	{".ts", model.LanguageTypeScript, "TypeScript"},
	{".js", model.LanguageJavaScript, "JavaScript"},
	{".rs", model.LanguageRust, "Rust"},
	{".py", model.LanguagePython, "Python"},
}

// detectRepoLanguages reports languages with at least one source file in the
// repo root or up to two directory levels deep (kept cheap on purpose).
func detectRepoLanguages() []langProbe {
	var found []langProbe
	for _, probe := range langProbes {
		for _, pattern := range []string{"*" + probe.ext, "*/*" + probe.ext, "*/*/*" + probe.ext} {
			if files, err := filepath.Glob(pattern); err == nil && len(files) > 0 {
				found = append(found, probe)
				break
			}
		}
	}
	return found
}
