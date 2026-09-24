package schema

type Symbol any

// Root contains things and states nothing itself.
type Root struct {
	Things []*Thing `eidos:"emit,walk"`
}

// Thing states a fact and contains any declaration.
type Thing struct {
	Async bool     `eidos:"emit,fact=Async"`
	Decls []Symbol `eidos:"emit,walk"`
	Name  string   `eidos:"both"`
}
