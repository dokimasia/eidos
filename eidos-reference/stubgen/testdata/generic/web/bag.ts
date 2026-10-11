// Bag keeps items of one type. Its method takes an item without a type.
//
//+reference:stub
export interface Bag<T> {
  add(item): void;
  take(): T;
}
