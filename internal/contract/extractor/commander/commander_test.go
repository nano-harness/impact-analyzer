package commander

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommanderExtractorExtract(t *testing.T) {
	tests := []struct {
		name, file, source string
		commands           []string
	}{
		{"single_line", "cli.js", `const { program } = require('commander'); program.command("a").option("-f").description("d");`, []string{"a"}},
		{"multiline", "cli.js", "program\n  .command('b')\n  .option('-x')\n  .description('d');", []string{"b"}},
		{"multiple", "cli.js", `program.command("a").option("-a"); program.command("b").option("-b");`, []string{"a", "b"}},
		{"ts_alias", "cli.ts", `import { program } from 'commander'; program.command("c").alias("cc").option("-c");`, []string{"c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&CommanderExtractor{}).Extract(path)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, c := range contracts {
				got[c.Identifier] = true
			}
			for _, command := range tt.commands {
				if !got[command] {
					t.Fatalf("missing %s, got %#v", command, got)
				}
			}
		})
	}
}
