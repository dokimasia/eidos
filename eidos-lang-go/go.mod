module go.dokimi.dev/eidos/lang/go

go 1.27.0

require (
	go.dokimi.dev/assert v0.0.0-20260930235119-12f31f1abf48
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/sdk v0.0.0-00010101000000-000000000000
	golang.org/x/mod v0.41.0
)

require (
	github.com/google/go-cmp v0.7.0 // indirect
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000 // indirect
)

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang

replace go.dokimi.dev/eidos/sdk => ../eidos-sdk
