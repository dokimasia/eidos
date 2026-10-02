module go.dokimi.dev/eidos/lang/rust

go 1.27.0

require (
	github.com/BurntSushi/toml v1.6.0
	go.dokimi.dev/assert v0.0.0-20260930235119-12f31f1abf48
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-rust v0.24.2 // indirect
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
