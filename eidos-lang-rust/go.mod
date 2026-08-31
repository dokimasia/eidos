module go.dokimi.dev/eidos/lang-rust

go 1.27.0

require (
	go.dokimi.dev/assert v0.0.0-20260831070102-e149efb43525
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000
	go.dokimi.dev/eidos/lang v0.0.0-00010101000000-000000000000
)

require github.com/google/go-cmp v0.7.0 // indirect

replace go.dokimi.dev/eidos/core => ../eidos-core

replace go.dokimi.dev/eidos/lang => ../eidos-lang
