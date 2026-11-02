;; Go symbols query - extracts functions, methods, types, interfaces

;; Function declarations with type parameters
(function_declaration
  name: (identifier) @function.name
  type_parameters: (type_parameter_list)? @function.type_params
  parameters: (parameter_list) @function.params
  result: (_)? @function.result) @function.def

;; Method declarations
(method_declaration
  receiver: (parameter_list) @method.receiver
  name: (field_identifier) @method.name
  parameters: (parameter_list) @method.params
  result: (_)? @method.result) @method.def

;; Type declarations (struct, interface) with type parameters
(type_declaration
  (type_spec
    name: (type_identifier) @type.name
    type_parameters: (type_parameter_list)? @type.type_params
    type: [
      (struct_type) @type.struct
      (interface_type) @type.interface
    ])) @type.def

;; Const declarations
(const_declaration
  (const_spec
    name: (identifier) @const.name
    value: (_)? @const.value)) @const.def

;; Var declarations
(var_declaration
  (var_spec
    name: (identifier) @var.name
    type: (_)? @var.type
    value: (_)? @var.value)) @var.def

;; Package declaration
(package_clause
  (package_identifier) @package.name) @package.def

;; Import declarations
(import_declaration
  (import_spec
    path: (interpreted_string_literal) @import.path)) @import.def

;; Anonymous functions / closures (#20)
;; Captures function literals (closures) for tracking
(func_literal
  parameters: (parameter_list) @closure.params
  result: (_)? @closure.result
  body: (block) @closure.body) @closure.def
