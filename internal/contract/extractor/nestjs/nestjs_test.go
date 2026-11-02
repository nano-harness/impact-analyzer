package nestjs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestJSExtractorExtract(t *testing.T) {
	tests := []struct {
		name, source string
		want         map[string]bool
	}{
		{"controller_prefix", `@Controller("api/v1") export class C { @Get("users") list(){} }`, map[string]bool{"GET /api/v1/users": true}},
		{"empty_paths", `@Controller() export class C { @Get() list(){} }`, map[string]bool{"GET /": true}},
		{"multiple_methods", `@Controller("api") export class C { @Get() @Post() list(){} }`, map[string]bool{"GET /api": true, "POST /api": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "c.ts")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&NestJSExtractor{}).Extract(path)
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
