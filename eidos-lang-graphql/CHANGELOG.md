# go.dokimi.dev/eidos/lang/graphql

## 0.1.3

### Patch Changes

- Updated dependencies [6294c29, 3d0b252, 6902587, 12e54f3]
  - go.dokimi.dev/eidos/core@0.6.0
  - go.dokimi.dev/eidos/sdk@0.4.0
  - go.dokimi.dev/eidos/lang@0.2.3

## 0.1.2

### Patch Changes

- Updated dependencies [3f4bf95]
  - go.dokimi.dev/eidos/core@0.5.0
  - go.dokimi.dev/eidos/lang@0.2.2
  - go.dokimi.dev/eidos/sdk@0.3.1

## 0.1.1

### Patch Changes

- Updated dependencies [b9677b7, e14100f, 575bf2a, 9621f52, 61f41a7, 7494b32, e64dff0]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/sdk@0.3.0
  - go.dokimi.dev/eidos/lang@0.2.1

## 0.1.0

### Minor Changes

- 3b46e8f: Add `graphql.ReservedPrefix` for the names that GraphQL reserves for its introspection.
- 3699aa0: Add the GraphQL satellite over `github.com/vektah/gqlparser/v2` v2.5.63: the frontend, the rules, the spoke, the backend and the toolchain adapter.
- 0d4b785: Implement the contracts of a sum on the object type of each variant.
- 3b46e8f: Read GraphQL through the fork `github.com/dokimasia/gqlparser/v2`.
- 3b46e8f: Read the descriptions of operations, fragments and variables as their documentation.
- 3b46e8f: Read the directives of a directive definition and of each `extend directive`.
- 0d4b785: Refuse a type parameter at every depth of a type.
- 0d4b785: Refuse a sum with contracts under the marker `graphql:input`.
- 3b46e8f: Refuse the well-known context, which a schema does not state.
- 0d4b785: Refuse the contracts of an enum.
- 0d4b785: Reject the fact `Overload`.
- 3b46e8f: Add the verdicts of the facts `Wire` and `Passing` to the coverage table of the GraphQL backend.
- 3b46e8f: Add the template helper `wrapdocs` and `backend.WrapDocs` to the GraphQL backend.
- d395e80: Write `extend directive` and the directives of a directive definition in the GraphQL backend.

### Patch Changes

- d2e5053: Require the fork `github.com/dokimasia/gqlparser/v2` at its commit 349414ad in place of the zero version.
- 3b46e8f: Refuse a wire name that is not a GraphQL name or that starts with `__` in the GraphQL lowering.
- 3b46e8f: Refuse the deleter of a property in the GraphQL lowering.
- 745cbd0: Fix the GraphQL backend to spell names without the initialisms of Go, so `user_id` becomes `userId` and `imageUrl` keeps its spelling.
- Updated dependencies [4ce5b02, eed0fb8, d5e68cb, 2a85f93, c53b949, cac3c9b, 09edb8b, 78909e0, c55b40a, 4ee3b36, 62d7b18, 6741030, c051021, 3561c92, 4520c2f, 097c95f, 32afe4b, 840a721, 19f2a4e]
  - go.dokimi.dev/eidos/lang@0.2.0
  - go.dokimi.dev/eidos/core@0.3.0
  - go.dokimi.dev/eidos/sdk@0.2.0
