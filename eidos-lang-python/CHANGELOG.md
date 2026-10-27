# go.dokimi.dev/eidos/lang/python

## 0.1.8

### Patch Changes

- 3bf527c: Raise the version of the backend for the plural s of an acronym.
- Updated dependencies [ec6b0c7]
  - go.dokimi.dev/eidos/lang@0.4.0

## 0.1.7

### Patch Changes

- Updated dependencies [c342cf5, 4f50e1a]
  - go.dokimi.dev/eidos/sdk@0.7.1
  - go.dokimi.dev/eidos/core@0.10.0
  - go.dokimi.dev/eidos/lang@0.3.3

## 0.1.6

### Patch Changes

- Updated dependencies [899b981, 8d1cae6, 729462f]
  - go.dokimi.dev/eidos/core@0.9.0
  - go.dokimi.dev/eidos/sdk@0.7.0
  - go.dokimi.dev/eidos/lang@0.3.2

## 0.1.5

### Patch Changes

- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.
- Updated dependencies [f2467a9, 0ccd4f4, b5864dc, 7559a1a, 53b3580, 6f125b4, 1833bae, 4702c35, 86dbe80]
  - go.dokimi.dev/eidos/core@0.8.0
  - go.dokimi.dev/eidos/sdk@0.6.0
  - go.dokimi.dev/eidos/lang@0.3.1

## 0.1.4

### Patch Changes

- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, 141d240, e1de752, 8c07b86, 529673e, 217c870, 7dfcd57, d0ab2c4, 6dc24ee, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0
  - go.dokimi.dev/eidos/sdk@0.5.0
  - go.dokimi.dev/eidos/lang@0.3.0

## 0.1.3

### Patch Changes

- 2296c3c: Fix the dependency round to pass over a need of the module of an output of the brand or of a package above it.
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

- e2fbfcb: Add the Python satellite, which reads and writes every declaration construct of Python 3 in modules and stub files.
- e176e64: Give each variant dataclass of a sum the contracts of the sum as its bases.
- e176e64: Mark a callable under the decorator `overload` as an overload signature.
- e176e64: Write the declarations of a module in the order that the module evaluates them.
- e176e64: Refuse the overload signatures of a method without an implementation in a module class that is not a Protocol.
- e2fbfcb: Refuse the well-known context, for which Python has no standard type.
- e176e64: Refuse the contracts of an enum.
- e176e64: Spell a type parameter by its name at every depth of a type.
- e176e64: Write the decorator `overload` for an overload signature.

### Patch Changes

- Updated dependencies [4ce5b02, eed0fb8, d5e68cb, 2a85f93, c53b949, cac3c9b, 09edb8b, 78909e0, c55b40a, 4ee3b36, 62d7b18, 6741030, c051021, 3561c92, 4520c2f, 097c95f, 32afe4b, 840a721, 19f2a4e]
  - go.dokimi.dev/eidos/lang@0.2.0
  - go.dokimi.dev/eidos/core@0.3.0
  - go.dokimi.dev/eidos/sdk@0.2.0
