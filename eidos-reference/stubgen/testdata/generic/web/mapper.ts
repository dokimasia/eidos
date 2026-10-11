// Mapper maps values of one type. A call record of its method cannot
// state the type of U.
//
//+reference:stub
export interface Mapper<T> {
  map<U>(v: T, f: (t: T) => U): U;
}
