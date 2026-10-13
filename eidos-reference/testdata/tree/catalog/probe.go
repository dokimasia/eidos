package catalog

// Probe checks the items of the catalog.
//
//+reference:stub
type Probe interface {
	// Check checks the item at the position append of the catalog.
	Check(item string, append int) error
}
