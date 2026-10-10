// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The lock's two files in a ledger's directory: the file the operating
// system locks, and the record of the holder that a contender reads.
const (
	lockName   = "lock"
	holderName = "lock.json"
)

// ErrLocked reports a ledger whose lock another holder has taken.
var ErrLocked = errors.New("ledger: the state directory is locked")

// errBusy reports a lock file that another open file or handle has
// locked. [Dir.Lock] turns it into a [LockedError] that names the holder.
var errBusy = errors.New("ledger: another open file has the lock")

// Locker is a ledger that admits one holder at a time, such as one run
// over a state directory. [Dir] and [Mem] implement it.
type Locker interface {
	// Lock takes the ledger's lock for h, and returns the function that
	// releases it. Lock does not wait: where another holder has the lock,
	// it returns a *[LockedError] that names that holder.
	//
	// Error modes: a *LockedError, which wraps [ErrLocked], the context's
	// error, and the operating system's error where the lock's files cannot
	// be created, opened or written.
	Lock(ctx context.Context, h Holder) (release func() error, err error)
}

// Holder is the record of a lock's holder.
//
// # Allocation contract
//
// [Holder.String] allocates the text it returns.
type Holder struct {
	// PID is the holder's process ID, and zero in the record of no
	// holder.
	PID int `json:"pid"`
	// Host is the name of the holder's host, and empty where the
	// operating system did not report one.
	Host string `json:"host"`
	// Caller describes the caller, such as the command line of a run.
	Caller string `json:"caller"`
	// Since is when the holder took the lock.
	Since time.Time `json:"since"`
}

// String describes the holder for a person: its process, its host, the
// time it took the lock in UTC, and its caller. The zero Holder is
// "another process".
func (h Holder) String() string {
	if h.PID == 0 {
		return "another process"
	}
	return fmt.Sprintf("process %d on %s since %s (%s)",
		h.PID, h.Host, h.Since.UTC().Format(time.RFC3339), h.Caller)
}

// LockedError reports the holder of a lock that [Locker.Lock] could not
// take.
type LockedError struct {
	// Holder is the holder's record, and the zero Holder where the record
	// does not read.
	Holder Holder
}

// Error states the holder, as [Holder.String] describes it.
func (e *LockedError) Error() string { return fmt.Sprintf("%v by %v", ErrLocked, e.Holder) }

// Unwrap returns [ErrLocked].
func (*LockedError) Unwrap() error { return ErrLocked }
