package schema

type Thing struct {
	Name string `eidos:"both"`
}

type Alias = Thing
