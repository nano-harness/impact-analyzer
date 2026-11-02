package express

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpressExtractorExtract(t *testing.T) {
	tests := []struct {
		name, file, source string
		want               map[string]bool
	}{
		{"basic", "app.js", `const express = require('express'); const app = express(); app.get("/x", h);`, map[string]bool{"GET /x": true}},
		{"router_prefix", "app.js", `const express = require('express'); const app = express(); const router = express.Router(); app.use("/api", router); router.post("/y", h);`, map[string]bool{"POST /api/y": true}},
		{"route_chain", "app.js", `const app = express(); app.route("/r").get(h).post(h);`, map[string]bool{"GET /r": true, "POST /r": true}},
		{"nested_router", "app.js", `const app = express(); const v1 = express.Router(); const v2 = express.Router(); app.use("/api", v1); v1.use("/sub", v2); v2.get("/z", h);`, map[string]bool{"GET /api/sub/z": true}},
		{"typescript_arrow", "app.ts", `import express from 'express'; const app = express(); app.get("/ts", (req, res) => res.send());`, map[string]bool{"GET /ts": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&ExpressExtractor{}).Extract(path)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, c := range contracts {
				got[c.Identifier] = true
			}
			for id := range tt.want {
				if !got[id] {
					t.Fatalf("missing %s, got %#v", id, got)
				}
			}
		})
	}
}
