//go:build !windows

package catalog

// Host describes the host that serves the catalog.
//
//+reference:builder
type Host struct {
	// Name is the name of the host.
	Name string
}
