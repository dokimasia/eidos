package schema

type Thing struct {
	Parts []*Part `eidos:"both,walk,owner"`
}

type Part struct {
	Name string `eidos:"both"`
}
