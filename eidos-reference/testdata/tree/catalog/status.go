package catalog

// Status is the state of a product in the catalog.
//
//+reference:enum
type Status string

// The states of a product.
const (
	// StatusDraft is a product that the catalog does not list.
	StatusDraft Status = "draft"
	// StatusListed is a product that the catalog lists.
	StatusListed Status = "listed"
	// StatusRetired is a product that the catalog no longer sells.
	StatusRetired Status = "retired"
)
