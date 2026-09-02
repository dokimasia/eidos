module go.dokimi.dev/eidos/sdk

go 1.27.0

require (
	go.dokimi.dev/assert v0.0.0-20260902112452-9d6eca9d7234
	go.dokimi.dev/eidos/core v0.0.0-00010101000000-000000000000
)

require github.com/google/go-cmp v0.7.0 // indirect

replace go.dokimi.dev/eidos/core => ../eidos-core
