// Mapper maps values of one type.
//
//+reference:mock
export interface Mapper<T> {
  // map returns the result of f for the value v.
  map<U>(v: T, f: (t: T) => U): U;
}
