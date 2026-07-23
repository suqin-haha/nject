## Chain analysis

1. Find the function parameter at the requested LSP position.
2. Discover workspace package metadata with `go/packages`.
3. Stop early unless the selected package transitively imports
   `github.com/muir/nject/v2`.
4. Type-check only workspace packages in that dependency subgraph.
5. Find `nject.Run` and `nject.MustRun` roots.
6. Resolve provider expressions through common nject wrappers, collection
   constructors, variables, and helper-function parameters.
7. Confirm that the selected function reaches a Run root.
8. Starting with the selected parameter type, walk backward through the
   provider list using exact `go/types` identity. The closest preceding
   producer wins, matching nject's normal down-flow rule.
9. Recursively resolve the producer's own argument types and return all
   participating functions.

Reflective providers and arbitrary runtime collection construction are opaque.
The analyzer returns no speculative edge for those cases.
