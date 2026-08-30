package schema

type Thing struct {
	A []*Part `eidos:"both,walk,slot=parts"`
	B []*Part `eidos:"both,walk,slot=parts"`
}

type Part struct {
	Name string `eidos:"both"`
}
