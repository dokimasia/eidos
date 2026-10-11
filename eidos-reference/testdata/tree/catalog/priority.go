package catalog

// Priority is the urgency of an order in the catalog.
//
//+reference:enum
type Priority uint64

// The priorities of an order.
const (
	// PriorityLow is an order that waits.
	PriorityLow Priority = 1
	// PriorityHigh is an order that goes first.
	PriorityHigh Priority = 1 << 63
)
