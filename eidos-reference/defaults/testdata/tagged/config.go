// Package tagged declares a struct whose field has a default under two tag
// keys.
package tagged

// Config is the configuration of a service.
type Config struct {
	// Region is the region of the service.
	Region string `fallback:"west" default:"east"`
}
