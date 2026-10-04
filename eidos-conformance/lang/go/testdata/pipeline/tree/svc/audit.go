package svc

import "context"

// audit records one call through a store. The audit weaver calls it
// first in every generated store method.
func audit(ctx context.Context) {}
