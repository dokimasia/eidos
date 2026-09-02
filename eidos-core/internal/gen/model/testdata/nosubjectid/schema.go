package schema

// Thing is a dispatch subject that cannot be addressed.
//
//eidos:subject
type Thing struct {
	Name string `eidos:"both"`
}
