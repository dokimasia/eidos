package cross_package_ref

import "f/cross_package_ref/dep"

type Holder struct {
	f0 dep.Target
}
