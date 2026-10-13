// Package tagged declares a struct whose field has a default under two tag
// keys, and a field whose tag has an empty default.
package tagged

// Config is the configuration of a service.
type Config struct {
	// Region is the region of the service.
	Region string `fallback:"west" default:"east"`
	// Zone is the zone of the service.
	Zone string `default:""`
}
