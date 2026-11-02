# TreeSitter Query Improvements (P1-D/E)

## Current Status

The impact-analyzer uses TreeSitter queries to extract symbols and call relationships from source files. The current implementation provides basic functionality but has limitations.

## Current Capabilities

### Go (`internal/lang/golang/go_symbols.scm`)
- ✅ Function declarations
- ✅ Method declarations
- ✅ Type declarations (struct, interface)
- ✅ Const declarations
- ✅ Var declarations
- ✅ Package declarations
- ✅ Import declarations

### Java (`internal/lang/java/java_symbols.scm`)
- ✅ Class declarations
- ✅ Interface declarations
- ✅ Method declarations (with modifiers)
- ✅ Constructor declarations
- ✅ Field declarations
- ✅ Package declarations
- ✅ Import declarations

### JavaScript/TypeScript (`internal/lang/javascript/js_symbols.scm`, `internal/lang/typescript/ts_symbols.scm`)
- ✅ Function declarations
- ✅ Class declarations (incl. TS abstract classes)
- ✅ Method declarations (incl. TS class/interface methods with owner attribution)
- ✅ Arrow functions and function-valued variable declarations (e.g. `const App = () => ...`)
- ✅ Variable declarations
- ✅ Import/export declarations, re-exports (`export ... from`), `export default Name`
- ✅ TS-only: type aliases, enums, namespaces
- ✅ JSX component usage edges (`<Component/>`, `.jsx`/`.tsx` files)

### Python (`internal/lang/python/py_symbols.scm`)
- ✅ Function/class declarations, decorators, `__all__` export detection
- ✅ Class inheritance edges from base classes (plain and `module.Base` qualified)
- ✅ Module-level call-bound variables (e.g. `app = FastAPI()`)

### Rust (`internal/lang/rust/adapter.go`, manual AST walk)
- ✅ Functions, structs, traits, modules, use declarations
- ✅ Enums (with variants), type aliases, consts/statics
- ✅ `impl Trait for Type` inheritance edges, `#[cfg(test)]` test detection

## Known Limitations

### 1. IsExported Detection

**Go:**
- ✅ **Working**: Uses naming convention (uppercase first letter)
- Implementation in `adapter.go:191-198`

**Java:**
- ⚠️ **Partial**: Requires modifier context from TreeSitter
- Current: Checks for "public" string in modifiers
- Limitation: Not all modifiers are captured in all contexts

**JavaScript/TypeScript:**
- ✅ **Implemented**: Export statements, `export default` (declaration and identifier forms),
  and re-export sources are captured and feed the reverse-import index
- Remaining challenge: indirect patterns (computed exports, CommonJS `module.exports`)

### 2. Import Path Resolution

**All Languages:**
- ⚠️ **Limited**: Captures import strings but doesn't resolve to actual files
- Impact: Dependency graphs show import strings, not file paths
- Needed: Import resolution logic per language ecosystem

**Go:**
- Package paths are captured correctly
- Would benefit from go/packages integration (now available via godeep)

**Java:**
- Fully qualified class names captured
- Would need classpath resolution

**JavaScript/TypeScript:**
- Relative and module paths captured
- Would need Node.js module resolution

### 3. Call Site Context

**All Languages:**
- ⚠️ **Basic**: Captures function/method names but not full context
- Missing: Type information, receiver types, generic instantiations
- Impact: Cannot distinguish overloaded methods

### 4. Generic/Template Support

**Go:**
- ❌ **Missing**: No support for type parameters
- Needed: Capture type parameter lists and constraints

**Java:**
- ❌ **Missing**: No generic type capture
- Needed: Type parameter extraction

**TypeScript:**
- ❌ **Missing**: No generic type capture
- Needed: Type parameter and constraint extraction

## Proposed Improvements

### Phase 1: Enhanced Symbol Queries

#### Go
```scheme
;; Add type parameters for generics
(function_declaration
  type_parameters: (type_parameter_list) @function.type_params
  name: (identifier) @function.name
  ...)

;; Capture receiver type for methods
(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      type: (_) @method.receiver_type))
  ...)
```

#### Java
```scheme
;; Better modifier capture
(method_declaration
  (modifiers
    (marker_annotation)* @method.annotations
    (modifier)* @method.modifiers)
  ...)

;; Generic type parameters
(class_declaration
  (type_parameters) @class.type_params
  ...)
```

#### JavaScript/TypeScript
```scheme
;; Export detection
(export_statement
  declaration: (function_declaration
    name: (identifier) @export.function))

(export_statement
  (export_specifier
    name: (identifier) @export.name))
```

### Phase 2: Import Resolution

Create import resolvers for each language:

**Go**: Use `golang.org/x/tools/go/packages` (✅ Now available via `godeep`)
**Java**: Parse classpaths and implement package resolution
**JavaScript/TypeScript**: Use Node.js resolution algorithm or TypeScript compiler API

### Phase 3: Type-Aware Analysis

**Go**: ✅ **Implemented** via `godeep` package
- Uses `go/types` for type information
- RTA algorithm for type-precise call graph
- Interface implementation detection

**Java/JavaScript/TypeScript**: Future work
- Would require language-specific type systems
- Consider using language servers (LSP)

## Migration Path

### Current (TreeSitter Only)
```
Source File → TreeSitter Parse → Symbols + Calls
```

### Future (Dual-Layer)
```
Source File → TreeSitter Parse (fast, all languages)
            ↓
            ├─→ Go: godeep analysis (type-precise)
            ├─→ Java: (future) Javac types
            └─→ JS/TS: (future) TypeScript compiler
```

## Implementation Priority

1. **High Priority** (Foundation exists)
   - ✅ Go deep analysis via godeep/RTA
   - ⬜ Import path resolution for dependencies

2. **Medium Priority** (Improves accuracy)
   - ✅ Enhanced export detection (JS/TS)
   - ⬜ Better modifier capture (Java)
   - ⬜ Generic type support (all languages)

3. **Low Priority** (Nice to have)
   - ⬜ Annotation/decorator capture
   - ⬜ Doc comment extraction
   - ⬜ Symbol metadata (deprecation, etc.)

## Technical Debt

### TreeSitter Query Complexity
- Current queries are simple pattern matches
- Complex language features (closures, lambdas, dynamic dispatch) not fully supported
- Trade-off: Simplicity vs. Completeness

### Test Coverage
- ✅ Go adapter: Comprehensive tests
- ✅ Java adapter: Comprehensive tests
- ✅ JavaScript adapter: Comprehensive tests (arrow/var symbols, re-exports, JSX edges, test-file detection)
- ✅ TypeScript adapter: Comprehensive tests (methods, type aliases, abstract classes, enums, namespaces, TSX edges)
- ✅ Python adapter: Comprehensive tests (inheritance edges, module-level bindings)
- ✅ Rust adapter: Comprehensive tests (enums, trait impls, `#[cfg(test)]` detection)

## Recommendations

### For P1-D/E Implementation:
1. **Don't** attempt to make TreeSitter queries perfect
2. **Do** focus on the dual-layer architecture
3. **Leverage** godeep for Go (already done)
4. **Document** limitations clearly (this doc)
5. **Prioritize** import resolution over query perfection

### For Users:
- Go projects: Enable deep analysis via config
- Java/JS/TS projects: Understand syntax-level limitations
- Consider contributing language-specific improvements

## References

- TreeSitter Query Syntax: https://tree-sitter.github.io/tree-sitter/using-parsers#pattern-matching-with-queries
- Go AST: https://pkg.go.dev/go/ast
- golang.org/x/tools: https://pkg.go.dev/golang.org/x/tools
- Java Language Spec: https://docs.oracle.com/javase/specs/
- TypeScript Compiler API: https://github.com/microsoft/TypeScript/wiki/Using-the-Compiler-API
