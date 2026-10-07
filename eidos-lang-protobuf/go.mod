module go.dokimi.dev/eidos/lang/protobuf

go 1.27.0

require (
	github.com/bufbuild/protocompile v0.14.1
	go.dokimi.dev/assert v0.0.0-20261007133442-6f235714117b
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
)

require (
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
