# go.dokimi.dev/eidos/cli

## 0.3.4

### Patch Changes

- Updated dependencies [510c973, dcbf4b0, 9996ac3, e938a44]
  - go.dokimi.dev/eidos/core@0.12.0
  - go.dokimi.dev/eidos/declarative@0.1.11

## 0.3.3

### Patch Changes

- 83dc1f9: Update `go.dokimi.dev/assert` to v0.10.0 and every other dependency to its latest release.
- Updated dependencies [71f1a16, 83dc1f9]
  - go.dokimi.dev/eidos/core@0.11.0
  - go.dokimi.dev/eidos/declarative@0.1.10

## 0.3.2

### Patch Changes

- Updated dependencies [4f50e1a]
  - go.dokimi.dev/eidos/core@0.10.0
  - go.dokimi.dev/eidos/declarative@0.1.9

## 0.3.1

### Patch Changes

- Updated dependencies [899b981, 8d1cae6]
  - go.dokimi.dev/eidos/core@0.9.0
  - go.dokimi.dev/eidos/declarative@0.1.8

## 0.3.0

### Minor Changes

- 8b056b3: Add the command `completion` of go-cli, which writes the completion script of a shell.

### Patch Changes

- 86dbe80: Update `go.dokimi.dev/assert` to v0.7.0.
- 8b056b3: Update `go.stealthscale.io/cli` to v0.7.0 and `go.stealthscale.io/cli/yaml` to v0.1.7.
- Updated dependencies [f2467a9, 0ccd4f4, 53b3580, 6f125b4, 86dbe80]
  - go.dokimi.dev/eidos/core@0.8.0
  - go.dokimi.dev/eidos/declarative@0.1.7

## 0.2.5

### Patch Changes

- e75c0ca: Require Go 1.27.3.
- Updated dependencies [9c9c4bb, e1de752, 8c07b86, 217c870, e75c0ca]
  - go.dokimi.dev/eidos/core@0.7.0
  - go.dokimi.dev/eidos/declarative@0.1.6

## 0.2.4

### Patch Changes

- Updated dependencies [6294c29, 3d0b252]
  - go.dokimi.dev/eidos/core@0.6.0
  - go.dokimi.dev/eidos/declarative@0.1.5

## 0.2.3

### Patch Changes

- Updated dependencies [3f4bf95]
  - go.dokimi.dev/eidos/core@0.5.0
  - go.dokimi.dev/eidos/declarative@0.1.4

## 0.2.2

### Patch Changes

- Updated dependencies [b9677b7, e14100f, 575bf2a]
  - go.dokimi.dev/eidos/core@0.4.0
  - go.dokimi.dev/eidos/declarative@0.1.3

## 0.2.1

### Patch Changes

- Updated dependencies [eed0fb8, d5e68cb, 2a85f93, 09edb8b, 78909e0, 6741030, 097c95f, 19f2a4e, ba70166]
  - go.dokimi.dev/eidos/core@0.3.0
  - go.dokimi.dev/eidos/declarative@0.1.2

## 0.2.0

### Minor Changes

- a2d0d64: Locate the stores of a frontend through `Input.Getenv` only when the tree has an input file of the frontend.
- a044621: Read the options of the command line from the config file of the workspace and from environment variables, and add `-C` to run a command in another directory.
- a044621: **Breaking:** Run the command line of a binary as an app of `go.stealthscale.io/cli`, with `New`, `Prepare`, `Mount`, `Main`, `Run`, `IOOf` and `Global` in place of `Kernels`, `Command`, `Flags` and `ProcessIO`.
- a044621: Render the error of a store locator in the pass of `watch` and run the next pass.

### Patch Changes

- 4644a2b: Update `go.dokimi.dev/assert` to v0.2.0.
- Updated dependencies [a2d0d64]
- Updated dependencies [a2d0d64]
- Updated dependencies [4644a2b]
  - go.dokimi.dev/eidos/core@0.2.0
  - go.dokimi.dev/eidos/declarative@0.1.1

## 0.1.0

### Minor Changes

- 2d40a7e: Add `cli.Run`, which runs the command line of a binary and returns the status of its command.
- 2d40a7e: Add `cli.ErrFormat`, which the error of `Format.Set` wraps.
- d69491d: Add `cli.Configure`, which applies the config file of a root to a composition.
- 2d40a7e: Check the JSON output of the command `schema` in `acceptancetest.AssertJSON`.
- 4ffc162: Add `Description.Disabled`, and list the plans that the config disables in `plan` and their plugins in `version`.
- 4ffc162: Add `Explanation.Declarations` and the event `explain.declaration` for the declarations that an identity or a key at a position names.
- 2d40a7e: **Breaking:** Exit `explain` with status 1 unless a workspace that contains the target records it in its generation.
- 96cb289: Load the declarative plugins of `plans.<name>.plugins` into an extensible plan, and report a plugin directory that does not load as a config error.
- 2d40a7e: Add `cli.ConfigName` and `cli.ConfigError` for the name and the faults of the config file of a brand.
- d7c134f: **Breaking:** Rename `workspace.RecordKind`, its constants, `ExplainedRecord` and `Explanation.Records` to the `Execution` names, and the event `explain.record` to `explain.execution`.
- d69491d: **Breaking:** Write the member of a list in the JSON field `member` and the summary list `members`, and rename `Renderer.Workspace` to `Renderer.Member`.
- d7c134f: **Breaking:** Rename the `Bucket` fields of `meta.Claim`, the phase contexts, `workspace.Component` and `plugintest.Fixture` to `Turn`, and print the field `turn` in `plan` and `explain`.
- 2d40a7e: Print the refinements of each plan in `plan` and the route of each unit of a file in `explain`.
- 2d40a7e: Print the brand in the text and the JSON output of `version`.
- d69491d: Print the contract version in `version`.
- 2d40a7e: Add the kernel command `schema` and `cli.Schema`, which print and return the JSON Schema of the config file with the options of each plugin.
- 2d40a7e: Read the settings of the commands of a binary from the key `extensions` of the config file through `Member.Extension`.
- 2d40a7e: Read the refinements of the generators and the families of a plan from `layout.generators` of the config file.
- 2d40a7e: Add the flag `--quiet`, which renders the Error findings alone and still counts every finding.
- 2d40a7e: Add `run --stdout`, which writes the generated files to standard output in place of the workspace.

### Patch Changes

- cd47b5c: Accept an absolute pattern of `run` and target of `explain` through a symbolic link to the workspace root.
- a074cab: Build the composition once for a workspace outside a list, where `Open` built it once for the brand and once for the workspace.
- 2d40a7e: Compare plan inside a repository with plan in a repository without a config file in `acceptancetest.AssertDiscovery`.
- d69491d: Compare the manifest in `acceptancetest.AssertIdempotent`, and run each process and lock of the kit under the context of the test.
- 2d40a7e: List a frontend and a backend that share a name as two plugins in `version`.
- 1915c6e: Name a config file relative to the workspace root in the error of a config that does not decode.
- d7c134f: Require `go.dokimi.dev/assert` v0.1.1.
- c7f0b83: Return an error from `ProcessIO` for a removed working directory on macOS.
- 56e2119: Start the `Usage` text of each kernel command with the form of the command, without the `usage:` prefix.
- Updated dependencies [8a44142]
- Updated dependencies [8a44142]
- Updated dependencies [79553b5]
- Updated dependencies [4ffc162]
- Updated dependencies [c045cd4]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [81c58aa]
- Updated dependencies [4ffc162]
- Updated dependencies [2bac013]
- Updated dependencies [d7c134f]
- Updated dependencies [e28d437]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [354ba3d]
- Updated dependencies [d7c134f]
- Updated dependencies [69477f7]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [42fa23a]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [576db82]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [8a44142]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [5c2df17]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [1f081a5]
- Updated dependencies [38284d3]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [7646457]
- Updated dependencies [8a44142]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4fd2025]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [012e4b8]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [85d4b42]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [f4aa524]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [1ce59e5]
- Updated dependencies [60bf040]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [012e4b8]
- Updated dependencies [4ffc162]
- Updated dependencies [4ffc162]
- Updated dependencies [d7c134f]
- Updated dependencies [d7c134f]
- Updated dependencies [4ffc162]
- Updated dependencies [9ff596a]
- Updated dependencies [4ffc162]
  - go.dokimi.dev/eidos/core@0.1.0
  - go.dokimi.dev/eidos/declarative@0.1.0
