package springmvc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpringMVCExtractorExtract(t *testing.T) {
	tests := []struct {
		name, source string
		want         map[string]bool
	}{
		{"simple_get_mapping", `@RestController class C { @GetMapping("/users") public void users(){} }`, map[string]bool{"GET /users": true}},
		{"class_level_request_mapping", `@RestController @RequestMapping("/api") class C { @PostMapping("/users") void users(){} }`, map[string]bool{"POST /api/users": true}},
		{"array_paths", `@RestController class C { @GetMapping({"/a","/b"}) void x(){} }`, map[string]bool{"GET /a": true, "GET /b": true}},
		{"path_alias", `@RestController class C { @GetMapping(path="/x") void x(){} }`, map[string]bool{"GET /x": true}},
		{"request_mapping_with_method", `@RestController class C { @RequestMapping(value="/x", method=RequestMethod.POST) void x(){} }`, map[string]bool{"POST /x": true}},
		{"request_mapping_method_array", `@RestController class C { @RequestMapping(value="/x", method={RequestMethod.POST, RequestMethod.PUT}) void x(){} }`, map[string]bool{"POST /x": true, "PUT /x": true}},
		{"multiline_annotation", "@RestController class C { @GetMapping(\n path=\"/m\"\n) void x(){} }", map[string]bool{"GET /m": true}},
		{"mixed_class_and_method_arrays", `@RestController @RequestMapping({"/a","/b"}) class C { @GetMapping({"/x","/y"}) void x(){} }`, map[string]bool{"GET /a/x": true, "GET /a/y": true, "GET /b/x": true, "GET /b/y": true}},
		{"non_controller_file", `class C { @GetMapping("/x") void x(){} }`, map[string]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "C.java")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&SpringMVCExtractor{}).Extract(path)
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

func TestSpringMVCExtractorCanHandle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "C.java")
	if err := os.WriteFile(path, []byte(`@RestController class C {}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(&SpringMVCExtractor{}).CanHandle(path) {
		t.Fatal("expected CanHandle")
	}
}

func TestSpringMVCExtractorSignatureDetails(t *testing.T) {
	source := `@RestController @RequestMapping("/api") class UserController {
	@GetMapping("/users/{id}") public UserDto get(@PathVariable String id, @RequestParam Integer page) { return null; }
	@PostMapping("/users") public UserDto create(@RequestBody CreateUserRequest body) { return null; }
}`
	dir := t.TempDir()
	path := filepath.Join(dir, "UserController.java")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	contracts, err := (&SpringMVCExtractor{}).Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]interface{}{}
	for _, c := range contracts {
		byID[c.Identifier] = c.Signature
	}
	getSig := byID["GET /api/users/{id}"]
	if getSig == nil {
		t.Fatalf("missing GET contract, got %#v", byID)
	}
	if getSig["handler"] != "UserController.get" {
		t.Fatalf("handler = %#v", getSig["handler"])
	}
	if getSig["response"] != "UserDto" {
		t.Fatalf("response = %#v", getSig["response"])
	}
	params := getSig["params"].([]map[string]interface{})
	if len(params) != 2 {
		t.Fatalf("params = %#v", params)
	}
	if params[0]["source"] != "path" || params[1]["source"] != "query" {
		t.Fatalf("params sources = %#v", params)
	}
	postSig := byID["POST /api/users"]
	if postSig == nil {
		t.Fatalf("missing POST contract, got %#v", byID)
	}
	postParams := postSig["params"].([]map[string]interface{})
	if len(postParams) != 1 || postParams[0]["source"] != "body" {
		t.Fatalf("post params = %#v", postParams)
	}
}
