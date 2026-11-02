// Package all imports all language adapters to trigger their init() registrations.
package all

import (
	_ "github.com/nano-harness/impact-analyzer/internal/lang/golang"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/java"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/javascript"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/python"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/rust"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/typescript"
)
