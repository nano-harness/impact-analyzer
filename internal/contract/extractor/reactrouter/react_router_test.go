package reactrouter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReactRouterExtractorExtract(t *testing.T) {
	tests := []struct {
		name, file, source string
		want               map[string]bool
	}{
		{"ice_config", "routes.ts", `import { IRouterConfig } from 'ice'; const routerConfig: IRouterConfig[] = [{ path: '/', component: Home }, { path: '/cr/:id', component: CodeReviewDetail }];`, map[string]bool{"FRONTEND /": true, "FRONTEND /cr/:id": true}},
		{"rr6_browser", "routes.tsx", `const router = createBrowserRouter([{ path: '/users', element: <Users /> }]);`, map[string]bool{"FRONTEND /users": true}},
		{"rr6_hash", "routes.jsx", `createHashRouter([{ path: '/settings', lazy: () => import('./Settings') }]);`, map[string]bool{"FRONTEND /settings": true}},
		{"jsx_route", "App.jsx", `<Switch><Route exact path="/x" component={X}/></Switch>`, map[string]bool{"FRONTEND /x": true}},
		{"nested_children", "routes.ts", `const routes = createBrowserRouter([{ path: '/user', children: [{ path: ':id', component: UserDetail }] }]);`, map[string]bool{"FRONTEND /user": true, "FRONTEND /user/:id": true}},
		{"non_route", "util.ts", `export const x = 1;`, map[string]bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			contracts, err := (&ReactRouterExtractor{}).Extract(path)
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

func TestReactRouterExtractorCanHandle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.tsx")
	if err := os.WriteFile(path, []byte(`createBrowserRouter([])`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(&ReactRouterExtractor{}).CanHandle(path) {
		t.Fatal("expected CanHandle")
	}
}
