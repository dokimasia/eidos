package store

// ErrClosed reports a transaction that is finished.
var ErrClosed error

// Tx is a transaction whose callables declare their classifications.
type Tx struct{}

// Begin starts the transaction.
//
//+acme:contract tx role=begin
func (t *Tx) Begin() error { return nil }

// Commit finishes the transaction, which the deleter detector would not
// claim, so its shape is the author's alone.
//
//+acme:shape writer reads=Get
//+acme:mixin atomic read=Get
//+acme:contract tx role=commit closed=ErrClosed
func (t *Tx) Commit() error { return nil }

// Rollback abandons the transaction.
//
//+acme:contract tx role=rollback
func (t *Tx) Rollback() error { return nil }

// Get reads a value inside the transaction.
func (t *Tx) Get(id string) (Value, error) { return Value{}, nil }

// Overrides corrects the detectors.
type Overrides struct{}

// Remove removes a value, which the deleter detector claims, and its
// directive declares a writer.
//
//+acme:shape writer
func (o *Overrides) Remove(v Value) error { return nil }

// Put persists a value, and its directive drops the writer the detector
// stamps.
//
//+acme:meta drop=shape.writer
func (o *Overrides) Put(v Value) error { return nil }
