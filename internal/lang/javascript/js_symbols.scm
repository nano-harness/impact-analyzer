;; JavaScript symbols query - extracts functions, classes, variables

;; Function declarations
(function_declaration
  name: (identifier) @function.name
  parameters: (formal_parameters) @function.params
  body: (statement_block) @function.body) @function.def

;; Arrow functions / function expressions (variable assignments)
(lexical_declaration
  (variable_declarator
    name: (identifier) @arrow.name
    value: [(arrow_function) (function_expression)] @arrow.func)) @arrow.def

;; Class declarations
(class_declaration
  name: (identifier) @class.name
  body: (class_body) @class.body) @class.def

;; Method definitions
(method_definition
  name: (property_identifier) @method.name
  parameters: (formal_parameters) @method.params
  body: (statement_block) @method.body) @method.def

;; Variable declarations (only function-valued declarators are symbols)
(variable_declaration
  (variable_declarator
    name: (identifier) @var.name
    value: [(arrow_function) (function_expression)] @var.func)) @var.def

;; Export declarations - function
(export_statement
  declaration: (function_declaration
    name: (identifier) @export.function.name)) @export.function.def

;; Export declarations - class
(export_statement
  declaration: (class_declaration
    name: (identifier) @export.class.name)) @export.class.def

;; Export declarations - variable
(export_statement
  declaration: (lexical_declaration
    (variable_declarator
      name: (identifier) @export.var.name))) @export.var.def

;; Export default
(export_statement
  "default" @export.default.keyword
  declaration: (_) @export.default.value) @export.default.stmt

;; Export default identifier (export default foo;)
(export_statement
  "default"
  value: (identifier) @export.default.name)

;; Export specifiers (named exports)
(export_statement
  (export_clause
    (export_specifier
      name: (identifier) @export.specifier.name))) @export.specifier.def

;; Re-export sources (export * from './x', export { a } from './x')
(export_statement
  source: (string) @export.source) @export.def

;; Import declarations
(import_statement
  source: (string) @import.source) @import.def
