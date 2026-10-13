import type { Product } from './store';

// Orders counts the products of orders.
//
//+reference:stub
//+reference:mock
export interface Orders {
  // count returns the number of products that the filter selects.
  count(filter: { product: Product; limit: number }): number;
}
