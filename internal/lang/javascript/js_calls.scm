;; JavaScript calls query - extracts function and method calls

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

;; JSX component usage (capitalized names filtered in the adapter)
(jsx_opening_element
  name: [(identifier) (member_expression)] @jsx.name)

(jsx_self_closing_element
  name: [(identifier) (member_expression)] @jsx.name)
