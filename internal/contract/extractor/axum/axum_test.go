package axum

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAxumExtractorExtract(t *testing.T) {
	tests := []struct {
		name, source string
		want         map[string]bool
	}{
		{"basic", `use axum::{Router, routing::get}; fn h(){} fn app(){ let app = Router::new().route("/x", get(h)); }`, map[string]bool{"GET /x": true}},
		{"multi_method", `use axum::{Router, routing::{get, post}}; fn app(){ let app = Router::new().route("/x", get(h).post(h2)); }`, map[string]bool{"GET /x": true, "POST /x": true}},
		{"nest", `use axum::{Router, routing::get}; fn app(){ let v1 = Router::new().route("/x", get(h)); let app = Router::new().nest("/api", v1); }`, map[string]bool{"GET /api/x": true}},
		{"multiple_routes", `use axum::{Router, routing::{get, post}}; fn app(){ let app = Router::new().route("/x", get(h)).route("/y", post(h)); }`, map[string]bool{"GET /x": true, "POST /y": true}},
		{"non_router", `fn h() { println!("hi"); }`, map[string]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "main.rs")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&AxumExtractor{}).Extract(path)
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

func TestAxumExtractorOnFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "fixtures", "axum-mini", "src", "main.rs")
	contracts, err := (&AxumExtractor{}).Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range contracts {
		got[c.Identifier] = true
	}
	want := map[string]bool{
		"GET /health":           true,
		"GET /api/v1/items":     true,
		"POST /api/v1/items":    true,
		"GET /api/v1/items/:id": true,
		"PUT /api/v1/items/:id": true,
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("missing %s, got %#v", id, got)
		}
	}
}
