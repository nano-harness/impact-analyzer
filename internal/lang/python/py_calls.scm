;; Python calls query - extracts function/method calls

;; Function calls
(call
  function: (identifier) @call.function) @call.def

;; Method calls
(call
  function: (attribute
    object: (_) @call.object
    attribute: (identifier) @call.method)) @call.method_def

;; Await calls
(await
  (call
    function: (identifier) @call.await_function)) @call.await_def
