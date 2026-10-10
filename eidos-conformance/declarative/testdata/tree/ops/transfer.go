package ops

// Transfer moves cents from one account to another.
//
//+acme:audit level=warn sink=record
func Transfer(from, to string, cents int64) {}
