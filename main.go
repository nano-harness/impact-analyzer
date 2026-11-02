package main

import (
	"fmt"
	"os"

	"github.com/nano-harness/impact-analyzer/cmd"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/all" // trigger all adapter registrations
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	cmd.SetVersion(version)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
