;; Python symbols query - extracts functions, classes, imports

;; Function definitions (including async)
(function_definition
  name: (identifier) @function.name
  parameters: (parameters) @function.params
  body: (_) @function.body) @function.def

;; Class definitions
(class_definition
  name: (identifier) @class.name
  superclasses: (argument_list)? @class.superclasses
  body: (_) @class.body) @class.def

;; Import statements
(import_statement
  name: (dotted_name) @import.module) @import.def

;; from X import Y - absolute imports
(import_from_statement
  module_name: (dotted_name) @import.from_module) @import.from_def

;; from .X import Y - relative imports
(import_from_statement
  module_name: (relative_import) @import.from_relative) @import.from_relative_def

;; Decorators (#25)
(decorated_definition
  (decorator) @decorator.def
  definition: (_) @decorator.target)

;; Module-level assignments binding a call result (app = FastAPI())
(module
  (expression_statement
    (assignment
      left: (identifier) @variable.name
      right: (call) @variable.value)) @variable.def)
