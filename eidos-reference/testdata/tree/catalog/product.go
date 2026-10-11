// Package catalog contains the products of the shop.
package catalog

// Product is one product of the catalog.
//
//+reference:repo
//+reference:audit
type Product struct {
	// ID identifies the product.
	ID string
	// Name is the name of the product.
	Name string
	// Status is the state of the product.
	Status Status
}
