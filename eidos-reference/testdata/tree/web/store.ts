// Product is one product of the catalog.
export interface Product {
  id: string;
  name: string;
}

// Store stores the products of the catalog.
//
//+reference:stub
//+reference:mock
export interface Store {
  // get returns the product with the identifier id.
  get(id: string): Product;
  // put stores the product p.
  put(p: Product): void;
}
