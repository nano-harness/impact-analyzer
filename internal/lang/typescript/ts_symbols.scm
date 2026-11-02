;; TypeScript symbols query - extends JavaScript with type information

;; Function declarations
(function_declaration
  name: (identifier) @function.name
  type_parameters: (type_parameters)? @function.type_params
  parameters: (formal_parameters) @function.params
  return_type: (type_annotation)? @function.return_type
  body: (statement_block) @function.body) @function.def

;; Arrow functions / function expressions (variable assignments)
(lexical_declaration
  (variable_declarator
    name: (identifier) @arrow.name
    type: (type_annotation)? @arrow.type
    value: [(arrow_function) (function_expression)] @arrow.func)) @arrow.def

;; Class declarations
(class_declaration
  name: (type_identifier) @class.name
  type_parameters: (type_parameters)? @class.type_params
  body: (class_body) @class.body) @class.def

;; Abstract class declarations
(abstract_class_declaration
  name: (type_identifier) @class.name
  type_parameters: (type_parameters)? @class.type_params
  body: (class_body) @class.body) @class.def

;; Enum declarations (treated like classes, as in the Java adapter)
(enum_declaration
  name: (identifier) @enum.name
  body: (enum_body) @enum.body) @enum.def

;; Namespaces / internal modules
[
  (internal_module
    name: (_) @namespace.name) @namespace.def
  (module
    name: (_) @namespace.name) @namespace.def
]

;; Interface declarations
(interface_declaration
  name: (type_identifier) @interface.name
  type_parameters: (type_parameters)? @interface.type_params) @interface.def

;; Type alias declarations
(type_alias_declaration
  name: (type_identifier) @type_alias.name
  type_parameters: (type_parameters)? @type_alias.type_params
  value: (_) @type_alias.value) @type_alias.def

;; Method definitions
(method_definition
  name: (property_identifier) @method.name
  parameters: (formal_parameters) @method.params
  return_type: (type_annotation)? @method.return_type
  body: (statement_block) @method.body) @method.def

;; Method signatures (in interfaces)
(method_signature
  name: (property_identifier) @method_sig.name
  parameters: (formal_parameters) @method_sig.params
  type: (type_annotation)? @method_sig.type) @method_sig.def

;; Abstract method signatures (in abstract classes)
(abstract_method_signature
  name: (property_identifier) @method_sig.name
  parameters: (formal_parameters) @method_sig.params
  return_type: (type_annotation)? @method_sig.return_type) @method_sig.def

;; Variable declarations (only function-valued declarators are symbols)
(variable_declaration
  (variable_declarator
    name: (identifier) @var.name
    type: (type_annotation)? @var.type
    value: [(arrow_function) (function_expression)] @var.func)) @var.def

;; Export declarations - function
(export_statement
  declaration: (function_declaration
    name: (identifier) @export.function.name)) @export.function.def

;; Export declarations - class
(export_statement
  declaration: (class_declaration
    name: (type_identifier) @export.class.name)) @export.class.def

;; Export declarations - abstract class
(export_statement
  declaration: (abstract_class_declaration
    name: (type_identifier) @export.class.name)) @export.class.def

;; Export declarations - interface
(export_statement
  declaration: (interface_declaration
    name: (type_identifier) @export.interface.name)) @export.interface.def

;; Export declarations - type alias
(export_statement
  declaration: (type_alias_declaration
    name: (type_identifier) @export.type.name)) @export.type.def

;; Export declarations - enum
(export_statement
  declaration: (enum_declaration
    name: (identifier) @export.enum.name)) @export.enum.def

;; Export declarations - namespace / module
(export_statement
  declaration: [
    (internal_module
      name: (_) @export.namespace.name)
    (module
      name: (_) @export.namespace.name)
  ]) @export.namespace.def

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
