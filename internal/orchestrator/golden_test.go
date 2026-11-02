package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goldenSyntheticChange is the fixed modification committed on top of the
// go-mini fixture's base commit, so HEAD~1 -> HEAD is a deterministic change:
// a signature change of the unexported greeter.message function (plus the
// Greet call-site update, keeping the module compilable).
const goldenSyntheticChange = `package greeter

import "fmt"

// Greet prints a greeting for the given name.
func Greet(name string) {
	fmt.Println(message(name, "hello"))
}

func message(name string, greeting string) string {
	return greeting + ", " + name
}
`

// TestGoldenBaselines verifies that changes don't unintentionally alter behavior.
// Each fixture is copied into a throwaway git repository (base commit + fixed
// synthetic change commit), so the golden is independent of the parent repo's
// history. Set UPDATE_GOLDEN=1 to regenerate baselines after intentional
// changes (see `make update-golden`).
func TestGoldenBaselines(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden baseline tests in short mode")
	}

	goldenDir := filepath.Join("..", "..", "testdata", "golden")
	fixturesDir := filepath.Join("..", "..", "testdata", "fixtures")

	if os.Getenv("UPDATE_GOLDEN") == "" {
		if _, err := os.Stat(goldenDir); os.IsNotExist(err) {
			t.Skip("Golden baseline directory does not exist yet - run `make update-golden` to create")
		}
	}

	fixtures := []string{"go-mini"}
	for _, fixtureName := range fixtures {
		t.Run(fixtureName, func(t *testing.T) {
			repo := testutil.NewMiniRepo(t)
			copyFixtureIntoRepo(t, filepath.Join(fixturesDir, fixtureName), repo)

			// Base commit: the fixture as checked in.
			repo.Commit(t, "base")
			// Change commit: fixed synthetic signature change.
			repo.WriteFile(t, "greeter/greeter.go", goldenSyntheticChange)
			repo.Commit(t, "change")

			cfg, err := config.LoadConfig()
			if err != nil {
				cfg = &config.Config{}
			}

			// Resolve symlinks (macOS temp dirs live under /private) so every
			// path in the result shares one prefix that normalization strips.
			repoDir, err := filepath.EvalSymlinks(repo.Dir)
			require.NoError(t, err)

			o, err := NewOrchestratorWithOptions(repoDir, filepath.Join(t.TempDir(), ".impact-cache"), cfg, nil)
			require.NoError(t, err)
			defer o.Close()

			result, err := o.AnalyzeDiff("HEAD~1", -1)
			require.NoError(t, err)

			actual := normalizeGoldenJSON(t, result, repoDir)
			goldenFile := filepath.Join(goldenDir, fixtureName+"-impact.json")

			if os.Getenv("UPDATE_GOLDEN") != "" {
				require.NoError(t, os.MkdirAll(goldenDir, 0755))
				require.NoError(t, os.WriteFile(goldenFile, actual, 0644))
				t.Logf("updated golden baseline %s", goldenFile)
				return
			}

			expectedData, err := os.ReadFile(goldenFile)
			if os.IsNotExist(err) {
				t.Skipf("Expected golden file does not exist: %s (run `make update-golden`)", goldenFile)
			}
			require.NoError(t, err)

			var expectedJSON, actualJSON map[string]interface{}
			require.NoError(t, json.Unmarshal(expectedData, &expectedJSON))
			require.NoError(t, json.Unmarshal(actual, &actualJSON))
			assert.Equal(t, expectedJSON, actualJSON, "Golden baseline mismatch")
		})
	}
}

// copyFixtureIntoRepo copies every regular file of the fixture directory into
// the mini repo, preserving relative paths.
func copyFixtureIntoRepo(t *testing.T, fixtureDir string, repo *testutil.MiniRepo) {
	t.Helper()
	require.DirExists(t, fixtureDir)
	err := filepath.WalkDir(fixtureDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixtureDir, path)
		if err != nil {
			return err
		}
		repo.WriteFile(t, rel, string(data))
		return nil
	})
	require.NoError(t, err)
}

// normalizeGoldenJSON marshals the result deterministically: the volatile
// timestamp is dropped and the repo-absolute path prefix is stripped so the
// golden is stable across machines and temp directories.
func normalizeGoldenJSON(t *testing.T, result interface{}, repoDir string) []byte {
	t.Helper()
	data, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)

	var asMap map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &asMap))
	delete(asMap, "generated_at")

	data, err = json.MarshalIndent(asMap, "", "  ")
	require.NoError(t, err)

	prefix := filepath.ToSlash(repoDir) + "/"
	return []byte(strings.ReplaceAll(string(data), prefix, ""))
}
