package clap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClapExtractorExtract(t *testing.T) {
	tests := []struct {
		name, source string
		want         map[string]bool
	}{
		{"basic_subcommand", `use clap::{Parser, Subcommand}; #[command(name="vwsd")] struct Cli{} #[derive(clap::Subcommand)] enum Cmd { Mount, Status }`, map[string]bool{"vwsd mount": true, "vwsd status": true}},
		{"variant_with_args", `use clap::Subcommand; #[command(name="vwsd")] struct Cli{} #[derive(Subcommand)] enum Cmd { Mount { #[arg(short)] path: String } }`, map[string]bool{"vwsd mount": true}},
		{"named_command", `use clap::{Parser, Subcommand}; #[command(name="myapp")] struct Cli{} #[derive(clap::Subcommand)] enum Cmd { Run }`, map[string]bool{"myapp run": true}},
		{"tuple_variant", `use clap::Subcommand; #[derive(Subcommand)] enum Cmd { Destroy(DestroyArgs) }`, map[string]bool{"destroy": true}},
		{"camel_case", `use clap::Subcommand; #[derive(Subcommand)] enum Cmd { MountPath, ListImages }`, map[string]bool{"mount-path": true, "list-images": true}},
		{"non_clap_file", `fn main(){}`, map[string]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "main.rs")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&ClapExtractor{}).Extract(path)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, c := range contracts {
				got[c.Identifier] = true
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %#v want %#v", got, tt.want)
			}
			for id := range tt.want {
				if !got[id] {
					t.Fatalf("missing %s, got %#v", id, got)
				}
			}
		})
	}
}

func TestClapExtractorCanHandle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.rs")
	if err := os.WriteFile(path, []byte(`use clap::Parser;`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(&ClapExtractor{}).CanHandle(path) {
		t.Fatal("expected CanHandle")
	}
}
