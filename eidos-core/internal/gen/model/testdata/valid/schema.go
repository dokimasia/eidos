package schema

type Symbol any

type Thing struct {
	Name     string   `eidos:"both"`
	Pos      int      `eidos:"node"`
	Async    bool     `eidos:"both,fact=Async"`
	Parts    []*Part  `eidos:"both,walk,slot=parts"`
	Decls    []Symbol `eidos:"node,walk"`
	Untagged string
}

type Part struct {
	Name string `eidos:"both"`
	Host Symbol `eidos:"both"`
}
