package catalog

// Cache keeps values of one type by key.
//
//+reference:stub
//+reference:mock
type Cache[T any] interface {
	// Get returns the value of key and whether key has one.
	Get(key string) (T, bool)
	// Put stores value under key.
	Put(key string, value T)
}
