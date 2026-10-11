package catalog

// Review is the review of a product by a customer.
//
//+reference:repo
//+reference:skip plugin=mockgen
type Review struct {
	// Product identifies the product of the review.
	Product string
	// Stars is the number of stars of the review, from one to five.
	Stars int
}
