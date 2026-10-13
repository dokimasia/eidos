package svc

// Ledger posts amounts to accounts.
//
//+eidos:audit level=info sink=record
type Ledger interface {
	// Post adds cents to account, and returns the identifier of the entry.
	Post(account string, cents int64) string
}
