package graph

import "testing"

func TestDepGraphDependenciesAndReverseDependencies(t *testing.T) {
	dg := NewDepGraph()
	dg.AddDependency("api", "service")
	dg.AddDependency("worker", "service")
	dg.AddDependency("service", "store")

	if got := dg.GetDependencies("api"); len(got) != 1 || got[0] != "service" {
		t.Fatalf("api deps=%v, want [service]", got)
	}
	reverse := dg.GetReverseDependencies("service")
	if len(reverse) != 2 {
		t.Fatalf("reverse service deps=%v, want api and worker", reverse)
	}
}
