;; TypeScript calls query - same as JavaScript

;; Function calls
(call_expression
  function: [
    (identifier) @call.function
    (member_expression
      object: (_) @call.object
      property: (property_identifier) @call.method)
  ]
  arguments: (arguments) @call.args) @call.expr

;; New expressions (constructor calls)
(new_expression
  constructor: (_) @call.constructor
  arguments: (arguments)? @call.args) @call.new
