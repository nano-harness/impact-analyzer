package contract

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestComputeDiffModifiedScenarios(t *testing.T) {
	tests := []struct {
		name            string
		oldC, newC      []*model.Contract
		added, modified int
	}{
		{"http_method_changes", []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x"})}, []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "POST", "path": "/x"})}, 0, 1},
		{"handler_changes", []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x", "handler": "old"})}, []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x", "handler": "new"})}, 0, 1},
		{"params_change", []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x", "params": []interface{}{}})}, []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x", "params": []interface{}{map[string]interface{}{"name": "id", "type": "String", "source": "query"}}})}, 0, 1},
		{"cli_flag_changes", []*model.Contract{testCLIContract("run", map[string]interface{}{"name": "run", "args": []interface{}{}})}, []*model.Contract{testCLIContract("run", map[string]interface{}{"name": "run", "args": []interface{}{map[string]interface{}{"name": "verbose", "type": "bool"}}})}, 0, 1},
		{"stable_signature", []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x"})}, []*model.Contract{testContract("GET /x", map[string]interface{}{"method": "GET", "path": "/x"})}, 0, 0},
		{"added_interface", []*model.Contract{}, []*model.Contract{testContract("GET /new", map[string]interface{}{"method": "GET", "path": "/new"})}, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff, err := ComputeDiff(tt.oldC, tt.newC)
			if err != nil {
				t.Fatal(err)
			}
			if len(diff.ContractsAdded) != tt.added || len(diff.ContractsModified) != tt.modified {
				t.Fatalf("added=%d modified=%d, want added=%d modified=%d diff=%#v", len(diff.ContractsAdded), len(diff.ContractsModified), tt.added, tt.modified, diff)
			}
		})
	}
}

func testContract(id string, sig map[string]interface{}) *model.Contract {
	c := &model.Contract{Kind: model.ContractKindHTTPRoute, Identifier: id, File: "api.go", Line: 1, Signature: sig, Language: model.LanguageGo}
	_ = c.ComputeSignatureHash()
	return c
}
func testCLIContract(id string, sig map[string]interface{}) *model.Contract {
	c := &model.Contract{Kind: model.ContractKindCLICommand, Identifier: id, File: "main.go", Line: 1, Signature: sig, Language: model.LanguageGo}
	_ = c.ComputeSignatureHash()
	return c
}
