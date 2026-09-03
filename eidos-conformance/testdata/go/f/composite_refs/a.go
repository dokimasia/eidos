package composite_refs

import "f/composite_refs/dep"

type Holder struct {
	f0 *dep.Target
	f1 map[string]dep.Target
	f2 func(dep.Target) error
}
