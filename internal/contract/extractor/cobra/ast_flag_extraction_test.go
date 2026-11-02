package cobra

import (
	"go/parser"
	"go/token"
	"testing"
)

func parseASTFlagFixture(t *testing.T, src string) []map[string]interface{} {
	t.Helper()
	node, err := parser.ParseFile(token.NewFileSet(), "cmd.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	return NewCobraExtractor().extractFlagsFromASTFile(node)
}

func flagsByName(flags []map[string]interface{}) map[string]string {
	result := map[string]string{}
	for _, flag := range flags {
		name, _ := flag["name"].(string)
		typ, _ := flag["type"].(string)
		result[name] = typ
	}
	return result
}

func TestExtractFlagsFromASTFile_AllVarTypes(t *testing.T) {
	flags := parseASTFlagFixture(t, `package cmd
import (
"time"
"github.com/spf13/cobra"
)
func newCmd() *cobra.Command {
var s string; var b bool; var i int; var i64 int64; var f64 float64; var ss []string; var sa []string; var d time.Duration; var c int
cmd := &cobra.Command{Use: "test"}
cmd.Flags().StringVar(&s, "string", "", "")
cmd.Flags().BoolVar(&b, "bool", false, "")
cmd.Flags().IntVar(&i, "int", 0, "")
cmd.Flags().Int64Var(&i64, "int64", 0, "")
cmd.Flags().Float64Var(&f64, "float64", 0, "")
cmd.Flags().StringSliceVar(&ss, "string-slice", nil, "")
cmd.Flags().StringArrayVar(&sa, "string-array", nil, "")
cmd.Flags().DurationVar(&d, "duration", 0, "")
cmd.Flags().CountVar(&c, "count", "")
return cmd
}`)
	byName := flagsByName(flags)
	want := map[string]string{"string": "String", "bool": "Bool", "int": "Int", "int64": "Int64", "float64": "Float64", "string-slice": "StringSlice", "string-array": "StringArray", "duration": "Duration", "count": "Count"}
	for name, typ := range want {
		if byName[name] != typ {
			t.Fatalf("flag %s type = %q, want %q", name, byName[name], typ)
		}
	}
}

func TestExtractFlagsFromASTFile_VarPVariants(t *testing.T) {
	flags := parseASTFlagFixture(t, `package cmd
import "github.com/spf13/cobra"
func newCmd() *cobra.Command {
var s string; var b bool; var i int
cmd := &cobra.Command{Use: "test"}
cmd.Flags().StringVarP(&s, "string", "s", "", "")
cmd.Flags().BoolVarP(&b, "bool", "b", false, "")
cmd.Flags().IntVarP(&i, "int", "i", 0, "")
return cmd
}`)
	byName := flagsByName(flags)
	if byName["string"] != "String" || byName["bool"] != "Bool" || byName["int"] != "Int" {
		t.Fatalf("unexpected VarP types: %v", byName)
	}
}

func TestExtractFlagsFromASTFile_PersistentFlags(t *testing.T) {
	flags := parseASTFlagFixture(t, `package cmd
import "github.com/spf13/cobra"
func newCmd() *cobra.Command {
var s string
cmd := &cobra.Command{Use: "test"}
cmd.PersistentFlags().StringVar(&s, "config", "", "")
return cmd
}`)
	byName := flagsByName(flags)
	if byName["config"] != "String" {
		t.Fatalf("config type = %q, want String", byName["config"])
	}
}

func TestExtractFlagsFromASTFile_DeduplicatesByName(t *testing.T) {
	flags := parseASTFlagFixture(t, `package cmd
import "github.com/spf13/cobra"
func newCmd() *cobra.Command {
var a string; var b string
cmd := &cobra.Command{Use: "test"}
cmd.Flags().StringVar(&a, "name", "", "")
cmd.PersistentFlags().StringVar(&b, "name", "", "")
return cmd
}`)
	if len(flags) != 1 {
		t.Fatalf("got %d flags, want 1: %v", len(flags), flags)
	}
	if flags[0]["name"] != "name" || flags[0]["type"] != "String" {
		t.Fatalf("unexpected flag: %v", flags[0])
	}
}
