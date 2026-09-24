package caller

import (
	"time"

	"example.test/fixture/clock"
)

func Now() time.Time { return clock.At(time.Now()) }
