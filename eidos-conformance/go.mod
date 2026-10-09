module go.dokimi.dev/eidos/conformance

go 1.27.0

require (
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/eidos/cli v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/go v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/java v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/protobuf v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/rust v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang/typescript v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/plugin/shape v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
)

require (
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/bufbuild/protocompile v0.14.1 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-java v0.23.5 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
	github.com/tree-sitter/tree-sitter-typescript v0.23.2 // indirect
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/mod v0.41.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace go.dokimi.dev/eidos/cli => ../eidos-cli

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/lang/go => ../eidos-lang-go

replace go.dokimi.dev/eidos/lang/java => ../eidos-lang-java

replace go.dokimi.dev/eidos/lang/protobuf => ../eidos-lang-protobuf

replace go.dokimi.dev/eidos/lang/rust => ../eidos-lang-rust

replace go.dokimi.dev/eidos/lang/typescript => ../eidos-lang-typescript

replace go.dokimi.dev/eidos/plugin/shape => ../eidos-plugin-shape

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
