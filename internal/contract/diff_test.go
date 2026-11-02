package contract

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestComputeDiffAddedRemovedModifiedAndStableImplementation(t *testing.T) {
	oldStable := diffTestContract("GET /users", map[string]interface{}{"method": "GET", "path": "/users", "handler": "Users"})
	newStable := diffTestContract("GET /users", map[string]interface{}{"method": "GET", "path": "/users", "handler": "Users"})
	oldRemoved := diffTestContract("GET /old", map[string]interface{}{"method": "GET", "path": "/old"})
	newAdded := diffTestContract("POST /users", map[string]interface{}{"method": "POST", "path": "/users"})
	oldModified := diffTestContract("GET /items", map[string]interface{}{"method": "GET", "path": "/items"})
	newModified := diffTestContract("GET /items", map[string]interface{}{"method": "POST", "path": "/items"})

	diff, err := ComputeDiff([]*model.Contract{oldStable, oldRemoved, oldModified}, []*model.Contract{newStable, newAdded, newModified})
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.ContractsAdded) != 1 || diff.ContractsAdded[0].Identifier != "POST /users" {
		t.Fatalf("added=%v, want POST /users", ids(diff.ContractsAdded))
	}
	if len(diff.ContractsRemoved) != 1 || diff.ContractsRemoved[0].Identifier != "GET /old" {
		t.Fatalf("removed=%v, want GET /old", ids(diff.ContractsRemoved))
	}
	if len(diff.ContractsModified) != 1 || diff.ContractsModified[0].Identifier != "GET /items" {
		t.Fatalf("modified=%v, want GET /items", ids(diff.ContractsModified))
	}
}

func TestSignatureHashIsStable(t *testing.T) {
	signature := map[string]interface{}{"path": "/users", "method": "GET", "params": []interface{}{map[string]interface{}{"name": "id"}}}
	first, err := model.HashSignature(signature)
	if err != nil {
		t.Fatal(err)
	}
	second, err := model.HashSignature(signature)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hashes differ: %s != %s", first, second)
	}
}

func diffTestContract(id string, sig map[string]interface{}) *model.Contract {
	c := &model.Contract{Kind: model.ContractKindHTTPRoute, Identifier: id, File: "api.go", Line: 1, Signature: sig, Framework: "test", Language: model.LanguageGo}
	_ = c.ComputeSignatureHash()
	return c
}

func ids(contracts []*model.Contract) []string {
	out := make([]string, len(contracts))
	for i, c := range contracts {
		out[i] = c.Identifier
	}
	return out
}
