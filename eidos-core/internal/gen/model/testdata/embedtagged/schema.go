package schema

type Base struct {
	Name string `eidos:"both"`
}

type Thing struct {
	Base `eidos:"both"`
}
