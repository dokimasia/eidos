package admin

// Auditor records what an operator changed.
//
//+acme:stub
type Auditor interface {
	Record(change string) error
}
