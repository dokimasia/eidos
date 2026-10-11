package catalog

// Item is one line of an order.
//
//+reference:register
type Item struct {
	// Product identifies the product of the line.
	Product string
	// Count is the number of products of the line.
	Count int
}
