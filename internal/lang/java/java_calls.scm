;; Java calls query - extracts method calls

;; Method invocations (object is optional: bare `find()` has no receiver)
(method_invocation
  object: (_)? @call.object
  name: (identifier) @call.method
  arguments: (argument_list) @call.args) @call.expr

;; Object creation (constructor calls)
(object_creation_expression
  type: (_) @call.constructor_type
  arguments: (argument_list) @call.args) @call.new

;; Explicit constructor invocations
(explicit_constructor_invocation
  arguments: (argument_list) @call.args) @call.super_this
