import type { Product } from './store';

// Cache keeps products of one kind by key.
//
//+reference:stub
//+reference:mock
export interface Cache<T extends Product> {
  // get returns the product with the key key.
  get(key: string): T | undefined;
  // put stores the product p under the key key.
  put(key: string, p: T): void;
}
