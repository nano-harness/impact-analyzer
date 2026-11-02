package fastapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestFastAPIExtractorExtract(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app.py")
	code := `from fastapi import FastAPI
app = FastAPI()
@app.get("/users")
def list_users():
    pass
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	ext := &FastAPIExtractor{}
	if !ext.CanHandle(file) {
		t.Fatal("expected CanHandle")
	}
	contracts, err := ext.Extract(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 1 || contracts[0].Kind != model.ContractKindHTTPRoute || contracts[0].Identifier != "GET /users" {
		t.Fatalf("unexpected contracts: %#v", contracts)
	}
}
