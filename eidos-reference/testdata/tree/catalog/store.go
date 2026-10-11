package catalog

import "context"

// Store stores the products of the catalog.
//
//+reference:stub
//+reference:mock
type Store interface {
	// Get returns the product with the identifier id.
	Get(ctx context.Context, id string) (*Product, error)
	// Put stores the product p.
	Put(ctx context.Context, p *Product) error
}
