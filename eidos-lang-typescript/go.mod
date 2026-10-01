module go.dokimi.dev/eidos/lang/typescript

go 1.27.0

require (
	github.com/tailscale/hujson v0.0.0-20260727124030-b80ff77dac4f
	go.dokimi.dev/assert v0.0.0-20260902112452-9d6eca9d7234
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tree-sitter/go-tree-sitter v0.25.0 // indirect
	github.com/tree-sitter/tree-sitter-typescript v0.23.2 // indirect
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
