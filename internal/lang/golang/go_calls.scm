;; Go calls query - extracts function and method calls

;; Function calls
(call_expression
  function: [
    (identifier) @call.function
    (selector_expression
      operand: (_) @call.operand
      field: (field_identifier) @call.method)
  ]
  arguments: (argument_list) @call.args) @call.expr

;; Selector expressions in non-call positions (method values like `f := s.Process`)
(selector_expression
  operand: (_) @selector.operand
  field: (field_identifier) @selector.field) @selector.expr

;; Type references used by Go orphan analysis.
(field_declaration
  type: (type_identifier) @type.ref) @type.expr

;; Field with qualified type (e.g., time.Time, context.Context) - #5
(field_declaration
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.field

(field_declaration
  type: (pointer_type
    (type_identifier) @type.ref)) @type.expr

;; Field with qualified pointer type - #5
(field_declaration
  type: (pointer_type
    (qualified_type
      package: (package_identifier) @typeref.pkg
      name: (type_identifier) @typeref.name))) @typeref.field

(composite_literal
  type: (type_identifier) @type.ref) @type.expr

;; Composite literal with qualified type - #5
(composite_literal
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.composite

(type_assertion_expression
  type: (type_identifier) @type.assertion) @type.expr

;; Type assertion with qualified type - #5
(type_assertion_expression
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.assert

(parameter_declaration
  type: (type_identifier) @type.ref) @type.expr

;; Parameter with qualified type - #5
(parameter_declaration
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.param

(parameter_declaration
  type: (pointer_type
    (type_identifier) @type.ref)) @type.expr

;; Parameter with qualified pointer type - #5
(parameter_declaration
  type: (pointer_type
    (qualified_type
      package: (package_identifier) @typeref.pkg
      name: (type_identifier) @typeref.name))) @typeref.param

(variadic_parameter_declaration
  type: (type_identifier) @type.ref) @type.expr

;; Variadic parameter with qualified type - #5
(variadic_parameter_declaration
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.variadic

(function_declaration
  result: (type_identifier) @type.ref) @type.expr

;; Function result with qualified type - #5
(function_declaration
  result: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.funcresult

(function_declaration
  result: (pointer_type
    (type_identifier) @type.ref)) @type.expr

;; Function result with qualified pointer type - #5
(function_declaration
  result: (pointer_type
    (qualified_type
      package: (package_identifier) @typeref.pkg
      name: (type_identifier) @typeref.name))) @typeref.funcresult

(method_declaration
  result: (type_identifier) @type.ref) @type.expr

;; Method result with qualified type - #5
(method_declaration
  result: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.methodresult

(method_declaration
  result: (pointer_type
    (type_identifier) @type.ref)) @type.expr

;; Method result with qualified pointer type - #5
(method_declaration
  result: (pointer_type
    (qualified_type
      package: (package_identifier) @typeref.pkg
      name: (type_identifier) @typeref.name))) @typeref.methodresult

(function_type
  result: (type_identifier) @type.ref) @type.expr

;; Function type result with qualified type - #5
(function_type
  result: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.functype

(function_type
  result: (pointer_type
    (type_identifier) @type.ref)) @type.expr

;; Function type result with qualified pointer type - #5
(function_type
  result: (pointer_type
    (qualified_type
      package: (package_identifier) @typeref.pkg
      name: (type_identifier) @typeref.name))) @typeref.functype

(pointer_type
  (type_identifier) @type.ref) @type.expr

;; Pointer to qualified type in any context (e.g., *pkg.Dep)
(pointer_type
  (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.pointer

;; Var/const declared types
(var_spec
  type: (type_identifier) @type.ref) @type.expr

(var_spec
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.var

(const_spec
  type: (type_identifier) @type.ref) @type.expr

(const_spec
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.const

;; Slice/array element types in any context
(slice_type
  element: (type_identifier) @type.ref) @type.expr

(slice_type
  element: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.slice

(array_type
  element: (type_identifier) @type.ref) @type.expr

(array_type
  element: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.array

;; Map key/value types in any context
(map_type
  key: (type_identifier) @type.ref) @type.expr

(map_type
  key: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.map

(map_type
  value: (type_identifier) @type.ref) @type.expr

(map_type
  value: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.map

;; Channel element types in any context
(channel_type
  value: (type_identifier) @type.ref) @type.expr

(channel_type
  value: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.chan

;; Generic instantiations (e.g., List[Dep], pkg.List[pkg.Dep])
(generic_type
  type: (type_identifier) @type.ref) @type.expr

(generic_type
  type: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.generic

;; Interface method results (params are covered by parameter_declaration)
(method_elem
  result: (type_identifier) @type.ref) @type.expr

(method_elem
  result: (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.method_elem

;; Embedded interface elements (e.g., type A interface { B })
(type_elem
  (type_identifier) @type.ref) @type.expr

(type_elem
  (qualified_type
    package: (package_identifier) @typeref.pkg
    name: (type_identifier) @typeref.name)) @typeref.type_elem
