;; Java symbols query - extracts classes, methods, interfaces

;; Class declarations with type parameters and inheritance
(class_declaration
  name: (identifier) @class.name
  type_parameters: (type_parameters)? @class.type_params
  superclass: (superclass (type_identifier) @class.superclass)?
  interfaces: (super_interfaces (type_list (type_identifier) @class.interface))?
  body: (class_body) @class.body) @class.def

;; Interface declarations with type parameters and inheritance
(interface_declaration
  name: (identifier) @interface.name
  type_parameters: (type_parameters)? @interface.type_params
  (extends_interfaces (type_list (type_identifier) @interface.extends))?
  body: (interface_body) @interface.body) @interface.def

;; Enum declarations
(enum_declaration
  (modifiers)? @enum.modifiers
  name: (identifier) @enum.name
  body: (enum_body) @enum.body) @enum.def

;; Record declarations
(record_declaration
  (modifiers)? @record.modifiers
  name: (identifier) @record.name
  type_parameters: (type_parameters)? @record.type_params
  body: (class_body) @record.body) @record.def

;; Method declarations with type parameters
(method_declaration
  (modifiers)? @method.modifiers
  type_parameters: (type_parameters)? @method.type_params
  type: (_)? @method.return_type
  name: (identifier) @method.name
  parameters: (formal_parameters) @method.params
  body: (_)? @method.body) @method.def

;; Constructor declarations
(constructor_declaration
  (modifiers)? @constructor.modifiers
  name: (identifier) @constructor.name
  parameters: (formal_parameters) @constructor.params
  body: (constructor_body) @constructor.body) @constructor.def

;; Field declarations
(field_declaration
  (modifiers)? @field.modifiers
  type: (_) @field.type
  declarator: (variable_declarator
    name: (identifier) @field.name)) @field.def

;; Package declaration
(package_declaration
  (scoped_identifier) @package.name) @package.def

;; Import declarations
(import_declaration
  (scoped_identifier) @import.path) @import.def

;; Anonymous class declarations (#23)
(object_creation_expression
  type: (type_identifier) @anon_class.type
  arguments: (argument_list)
  (class_body) @anon_class.body) @anon_class.def
