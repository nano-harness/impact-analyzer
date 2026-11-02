package orchestrator

// Labeled evaluation scenarios for TestEval_PrecisionRecall. All repositories
// are built in-memory via testutil.NewMiniRepo; no external fixtures.
//
// KnownFailure root causes reference these observed behaviors (verified by
// probes at the time of writing):

import "github.com/nano-harness/impact-analyzer/internal/model"

func evalScenarios() []evalScenario {
	return []evalScenario{
		// ------------------------------------------------------------------
		// Go
		// ------------------------------------------------------------------
		{
			Name:     "go-same-package-call",
			Language: "go",
			Notes:    "same-package caller must be recalled; uncalled function must not",
			InitialFiles: map[string]string{
				"go.mod": "module example.com/eval\n\ngo 1.25\n",
				"main.go": `package main

func Helper() string {
	return "a"
}

func Service() string {
	return Helper()
}

func Loose() string {
	return "loose"
}

func main() {
	Service()
}
`,
			},
			ChangedFiles: map[string]string{
				"main.go": `package main

func Helper() string {
	return "b"
}

func Service() string {
	return Helper()
}

func Loose() string {
	return "loose"
}

func main() {
	Service()
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "main", Name: "Service"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "main", Name: "Loose"}},
		},
		{
			Name:     "go-cross-package-import",
			Language: "go",
			Notes:    "importing caller in another package recalled; same-named function in a non-importing package must not be attributed",
			InitialFiles: map[string]string{
				"go.mod": "module example.com/eval\n\ngo 1.25\n",
				"lib/lib.go": `package lib

func Helper() string {
	return "a"
}
`,
				"svc/svc.go": `package svc

import "example.com/eval/lib"

func Service() string {
	return lib.Helper()
}
`,
				"third/third.go": `package third

func Helper() string {
	return "third"
}

func UseHelper() string {
	return Helper()
}
`,
			},
			ChangedFiles: map[string]string{
				"lib/lib.go": `package lib

func Helper() string {
	return "b"
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "svc", Name: "Service"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "third", Name: "UseHelper"}},
		},
		{
			Name:     "go-interface-method-sig-change",
			Language: "go",
			Notes:    "changing an interface method signature should flag implementations as affected and breaking",
			InitialFiles: map[string]string{
				"go.mod": "module example.com/eval\n\ngo 1.25\n",
				"svc/svc.go": `package svc

type Greeter interface {
	Greet(name string) string
}

type EnglishGreeter struct{}

func (EnglishGreeter) Greet(name string) string {
	return "hi " + name
}
`,
			},
			ChangedFiles: map[string]string{
				"svc/svc.go": `package svc

type Greeter interface {
	Greet(name string, punct string) string
}

type EnglishGreeter struct{}

func (EnglishGreeter) Greet(name string) string {
	return "hi " + name
}
`,
			},
			ExpectedAffected: []evalSymbolSpec{{Package: "svc", Name: "EnglishGreeter"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Package: "svc", Name: "Greeter"},
				MinLevel: model.RiskLevelBreaking,
				Reason:   "interface method signature change breaks all implementations",
			}},
		},
		{
			Name:     "go-body-only-modify",
			Language: "go",
			Notes:    "body-only change must recall callers but must not be breaking (multi-line function body)",
			InitialFiles: map[string]string{
				"go.mod": "module example.com/eval\n\ngo 1.25\n",
				"lib/lib.go": `package lib

func Helper() string {
	return "a"
}
`,
				"svc/svc.go": `package svc

import "example.com/eval/lib"

func Service() string {
	return lib.Helper()
}
`,
			},
			ChangedFiles: map[string]string{
				"lib/lib.go": `package lib

func Helper() string {
	return "b"
}
`,
			},
			ExpectedAffected: []evalSymbolSpec{{Package: "svc", Name: "Service"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Package: "lib", Name: "Helper"},
				MaxLevel: model.RiskLevelWarning,
				Reason:   "body-only change keeps the signature, so it cannot break callers",
			}},
		},
		{
			Name:     "go-single-line-body-only",
			Language: "go",
			Notes:    "single-line function shares its declaration line with the body; a body edit must not be misread as a signature change",
			InitialFiles: map[string]string{
				"go.mod": "module example.com/eval\n\ngo 1.25\n",
				"lib/lib.go": `package lib

func Add(a, b int) int { return a + b }
`,
				"svc/svc.go": `package svc

import "example.com/eval/lib"

func Service() int {
	return lib.Add(1, 2)
}
`,
			},
			ChangedFiles: map[string]string{
				"lib/lib.go": `package lib

func Add(a, b int) int { return a + b + 0 }
`,
			},
			ExpectedAffected: []evalSymbolSpec{{Package: "svc", Name: "Service"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Package: "lib", Name: "Add"},
				MaxLevel: model.RiskLevelWarning,
				Reason:   "single-line body-only change keeps the signature, so it cannot break callers",
			}},
		},

		// ------------------------------------------------------------------
		// Java
		// ------------------------------------------------------------------
		{
			Name:     "java-same-package-call",
			Language: "java",
			Notes:    "same-package method caller recalled; class with same-named method in the same package must not be attributed",
			InitialFiles: map[string]string{
				"src/main/java/com/example/lib/Helper.java": `package com.example.lib;

public class Helper {
    public String greet(String name) {
        return "hello " + name;
    }
}
`,
				"src/main/java/com/example/lib/Service.java": `package com.example.lib;

public class Service {
    private final Helper helper = new Helper();

    public String serve(String name) {
        return helper.greet(name);
    }
}
`,
				"src/main/java/com/example/lib/Other.java": `package com.example.lib;

public class Other {
    public String greet(String name) {
        return "other " + name;
    }

    public String useOwn(String name) {
        return greet(name);
    }
}
`,
			},
			ChangedFiles: map[string]string{
				"src/main/java/com/example/lib/Helper.java": `package com.example.lib;

public class Helper {
    public String greet(String name) {
        return "hi " + name;
    }
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "com.example.lib", Name: "serve"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "com.example.lib", Name: "useOwn"}},
		},
		{
			Name:     "java-cross-package-call",
			Language: "java",
			Notes:    "cross-package caller via import; same-named method in a non-importing package must not be attributed",
			InitialFiles: map[string]string{
				"src/main/java/com/example/lib/Helper.java": `package com.example.lib;

public class Helper {
    public String greet(String name) {
        return "hello " + name;
    }
}
`,
				"src/main/java/com/example/svc/Service.java": `package com.example.svc;

import com.example.lib.Helper;

public class Service {
    private final Helper helper = new Helper();

    public String serve(String name) {
        return helper.greet(name);
    }
}
`,
				"src/main/java/com/example/other/Other.java": `package com.example.other;

public class Other {
    public String greet(String name) {
        return "other " + name;
    }

    public String useOwn(String name) {
        return greet(name);
    }
}
`,
			},
			ChangedFiles: map[string]string{
				"src/main/java/com/example/lib/Helper.java": `package com.example.lib;

public class Helper {
    public String greet(String name) {
        return "hi " + name;
    }
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "com.example.svc", Name: "serve"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "com.example.other", Name: "useOwn"}},
		},
		{
			Name:     "java-interface-implements",
			Language: "java",
			Notes:    "implementing class must be affected (InterfaceImpls) and breaking when an interface method signature changes",
			InitialFiles: map[string]string{
				"src/main/java/com/example/api/Greeter.java": `package com.example.api;

public interface Greeter {
    String greet(String name);
}
`,
				"src/main/java/com/example/api/EnglishGreeter.java": `package com.example.api;

public class EnglishGreeter implements Greeter {
    @Override
    public String greet(String name) {
        return "hi " + name;
    }
}
`,
			},
			ChangedFiles: map[string]string{
				"src/main/java/com/example/api/Greeter.java": `package com.example.api;

public interface Greeter {
    String greet(String name, String punct);
}
`,
			},
			ExpectedAffected: []evalSymbolSpec{{Package: "com.example.api", Name: "EnglishGreeter"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Package: "com.example.api", Name: "greet"},
				MinLevel: model.RiskLevelBreaking,
				Reason:   "interface method signature change breaks all implementations",
			}},
		},

		// ------------------------------------------------------------------
		// TypeScript
		// ------------------------------------------------------------------
		{
			Name:     "ts-module-import-call",
			Language: "typescript",
			Notes:    "caller importing the changed module recalled; sibling function that does not call it must not be affected",
			InitialFiles: map[string]string{
				"src/lib.ts": `export function greet(name: string): string {
  return "hello " + name;
}
`,
				"src/svc.ts": `import { greet } from "./lib";

export function serve(name: string): string {
  return greet(name);
}

export function describe(name: string): string {
  return "user " + name;
}
`,
			},
			ChangedFiles: map[string]string{
				"src/lib.ts": `export function greet(name: string): string {
  return "hi " + name;
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Name: "serve", File: "src/svc.ts"}},
			ExpectedUnaffected: []evalSymbolSpec{{Name: "describe", File: "src/svc.ts"}},
		},
		{
			Name:     "ts-cross-module-same-name",
			Language: "typescript",
			Notes:    "same-named local function in a module that does not import the changed module must not be attributed",
			InitialFiles: map[string]string{
				"src/lib.ts": `export function greet(name: string): string {
  return "hello " + name;
}
`,
				"src/svc.ts": `import { greet } from "./lib";

export function serve(name: string): string {
  return greet(name);
}
`,
				"src/other.ts": `export function greet(name: string): string {
  return "other " + name;
}

export function useOwn(name: string): string {
  return greet(name);
}
`,
			},
			ChangedFiles: map[string]string{
				"src/lib.ts": `export function greet(name: string): string {
  return "hi " + name;
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Name: "serve", File: "src/svc.ts"}},
			ExpectedUnaffected: []evalSymbolSpec{{Name: "useOwn", File: "src/other.ts"}},
		},

		// ------------------------------------------------------------------
		// Python
		// ------------------------------------------------------------------
		{
			Name:     "python-module-import-call",
			Language: "python",
			Notes:    "caller importing the changed module recalled; body-only modify should not be breaking",
			InitialFiles: map[string]string{
				"lib.py": `def greet(name):
    return "hello " + name
`,
				"svc.py": `from lib import greet


def serve(name):
    return greet(name)


def describe(name):
    return "user " + name
`,
			},
			ChangedFiles: map[string]string{
				"lib.py": `def greet(name):
    return "hi " + name
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "svc", Name: "serve"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "svc", Name: "describe"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Name: "greet"},
				MaxLevel: model.RiskLevelWarning,
				Reason:   "body-only modify keeps the signature, so it cannot break callers",
			}},
		},
		{
			Name:     "python-cross-module-same-name",
			Language: "python",
			Notes:    "same-named function in a module that does not import the changed module must not be attributed",
			InitialFiles: map[string]string{
				"lib.py": `def greet(name):
    return "hello " + name
`,
				"svc.py": `from lib import greet


def serve(name):
    return greet(name)
`,
				"other.py": `def greet(name):
    return "other " + name


def use_own(name):
    return greet(name)
`,
			},
			ChangedFiles: map[string]string{
				"lib.py": `def greet(name):
    return "hi " + name
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "svc", Name: "serve"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "other", Name: "use_own"}},
		},

		// ------------------------------------------------------------------
		// Test mapping (Go)
		// ------------------------------------------------------------------
		{
			Name:     "go-test-mapping",
			Language: "go",
			Notes:    "test calling the changed function listed in affected_tests; same-named test in another package must not be listed",
			InitialFiles: map[string]string{
				"go.mod": "module example.com/eval\n\ngo 1.25\n",
				"lib/lib.go": `package lib

func Helper() string {
	return "a"
}
`,
				"lib/lib_test.go": `package lib

import "testing"

func TestHelper(t *testing.T) {
	if Helper() == "" {
		t.Fail()
	}
}
`,
				"other/other.go": `package other

func Helper() string {
	return "other"
}
`,
				"other/other_test.go": `package other

import "testing"

func TestHelper(t *testing.T) {
	if Helper() == "" {
		t.Fail()
	}
}
`,
			},
			ChangedFiles: map[string]string{
				"lib/lib.go": `package lib

func Helper() string {
	return "b"
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Package: "lib", Name: "TestHelper"}},
			ExpectedUnaffected: []evalSymbolSpec{{Package: "other", Name: "TestHelper"}},
			ExpectedTests:      []evalSymbolSpec{{Package: "lib", Name: "TestHelper"}},
			UnexpectedTests:    []evalSymbolSpec{{Package: "other", Name: "TestHelper"}},
		},

		// ------------------------------------------------------------------
		// Rust
		// ------------------------------------------------------------------
		{
			Name:     "rust-cross-module-import-call",
			Language: "rust",
			Notes:    "caller importing the changed function via `use crate::...` recalled; sibling function that does not call it must not be affected",
			InitialFiles: map[string]string{
				"Cargo.toml": `[package]
name = "eval"
version = "0.1.0"
edition = "2021"
`,
				"src/main.rs": `mod greeter;
mod svc;

fn main() {
    println!("{}", svc::serve("world"));
}
`,
				"src/greeter.rs": `pub fn greet(name: &str) -> String {
    format!("hello {name}")
}
`,
				"src/svc.rs": `use crate::greeter::greet;

pub fn serve(name: &str) -> String {
    greet(name)
}

pub fn describe(name: &str) -> String {
    format!("user {name}")
}
`,
			},
			ChangedFiles: map[string]string{
				"src/greeter.rs": `pub fn greet(name: &str) -> String {
    format!("hi {name}")
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Name: "serve", File: "src/svc.rs"}},
			ExpectedUnaffected: []evalSymbolSpec{{Name: "describe", File: "src/svc.rs"}},
		},
		{
			Name:     "rust-body-only-modify",
			Language: "rust",
			Notes:    "body-only change to a function called through a qualified crate:: path must recall callers but must not be breaking",
			InitialFiles: map[string]string{
				"Cargo.toml": `[package]
name = "eval"
version = "0.1.0"
edition = "2021"
`,
				"src/main.rs": `mod greeter;
mod svc;

fn main() {
    println!("{}", svc::serve("world"));
}
`,
				"src/greeter.rs": `pub fn greet(name: &str) -> String {
    format!("hello {name}")
}
`,
				"src/svc.rs": `pub fn serve(name: &str) -> String {
    crate::greeter::greet(name)
}

pub fn describe(name: &str) -> String {
    format!("user {name}")
}
`,
			},
			ChangedFiles: map[string]string{
				"src/greeter.rs": `pub fn greet(name: &str) -> String {
    format!("hi {name}")
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Name: "serve", File: "src/svc.rs"}},
			ExpectedUnaffected: []evalSymbolSpec{{Name: "describe", File: "src/svc.rs"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Name: "greet", File: "src/greeter.rs"},
				MaxLevel: model.RiskLevelWarning,
				Reason:   "body-only change keeps the signature, so it cannot break callers",
			}},
		},
		{
			Name:     "rust-cross-module-same-name",
			Language: "rust",
			Notes:    "same-named function in a module that does not import the changed module must not be attributed",
			InitialFiles: map[string]string{
				"Cargo.toml": `[package]
name = "eval"
version = "0.1.0"
edition = "2021"
`,
				"src/main.rs": `mod greeter;
mod other;
mod svc;

fn main() {
    println!("{}", svc::serve("world"));
}
`,
				"src/greeter.rs": `pub fn greet(name: &str) -> String {
    format!("hello {name}")
}
`,
				"src/svc.rs": `use crate::greeter::greet;

pub fn serve(name: &str) -> String {
    greet(name)
}
`,
				"src/other.rs": `pub fn greet(name: &str) -> String {
    format!("other {name}")
}

pub fn use_own(name: &str) -> String {
    greet(name)
}
`,
			},
			ChangedFiles: map[string]string{
				"src/greeter.rs": `pub fn greet(name: &str) -> String {
    format!("hi {name}")
}
`,
			},
			ExpectedAffected:   []evalSymbolSpec{{Name: "serve", File: "src/svc.rs"}},
			ExpectedUnaffected: []evalSymbolSpec{{Name: "use_own", File: "src/other.rs"}},
		},
		{
			Name:     "rust-impl-method-call",
			Language: "rust",
			Notes:    "impl-block method change recalls both a cross-module caller (receiver call) and a same-impl self caller; same-named method on another type must not be attributed",
			InitialFiles: map[string]string{
				"Cargo.toml": `[package]
name = "eval"
version = "0.1.0"
edition = "2021"
`,
				"src/main.rs": `mod audit;
mod user;

use crate::user::User;

fn main() {
    println!("{}", render("a"));
}

fn render(name: &str) -> String {
    let u = User::new(name);
    u.label()
}
`,
				"src/user.rs": `pub struct User {
    name: String,
}

impl User {
    pub fn new(name: &str) -> User {
        User {
            name: name.to_string(),
        }
    }

    pub fn label(&self) -> String {
        format!("user: {}", self.name)
    }

    pub fn label_loud(&self) -> String {
        format!("[{}]", self.label())
    }
}
`,
				"src/audit.rs": `pub struct Audit;

impl Audit {
    pub fn label(&self) -> String {
        "audit".to_string()
    }
}

pub fn render_audit(a: &Audit) -> String {
    a.label()
}
`,
			},
			ChangedFiles: map[string]string{
				"src/user.rs": `pub struct User {
    name: String,
}

impl User {
    pub fn new(name: &str) -> User {
        User {
            name: name.to_string(),
        }
    }

    pub fn label(&self) -> String {
        format!("member: {}", self.name)
    }

    pub fn label_loud(&self) -> String {
        format!("[{}]", self.label())
    }
}
`,
			},
			ExpectedAffected: []evalSymbolSpec{
				{Name: "render", File: "src/main.rs"},
				{Owner: "User", Name: "label_loud", File: "src/user.rs"},
			},
			ExpectedUnaffected: []evalSymbolSpec{{Name: "render_audit", File: "src/audit.rs"}},
			RiskChecks: []evalRiskCheck{{
				Change:   evalSymbolSpec{Owner: "User", Name: "label", File: "src/user.rs"},
				MaxLevel: model.RiskLevelWarning,
				Reason:   "body-only method change keeps the signature, so it cannot break callers",
			}},
		},
	}
}
