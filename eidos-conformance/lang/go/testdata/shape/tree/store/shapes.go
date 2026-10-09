// Package store declares one callable for each detected shape of the
// catalog, and the directives of the catalog.
package store

import (
	"context"
	"iter"
)

// Value is a stored value.
type Value struct {
	ID      string
	Version int
}

// Meta describes a stored value.
type Meta struct {
	Size int
}

// Source is a stream of values.
type Source interface {
	Next() (Value, bool)
}

// Shapes declares one method for each detected shape, in the order of the
// catalog's names.
type Shapes struct{}

func (s *Shapes) Count(ctx context.Context) (int, error) { return 0, nil }

func (s *Shapes) Store(ctx context.Context, v Value) (Value, error) { return v, nil }

func (s *Shapes) GetAll(ctx context.Context, ids ...string) ([]Value, error) { return nil, nil }

func (s *Shapes) Close() error { return nil }

func (s *Shapes) Set(ctx context.Context, key string, v Value) error { return nil }

func (s *Shapes) Add(a, b int) int { return a + b }

func (s *Shapes) Delete(ctx context.Context, id string) error { return nil }

func (s *Shapes) Start(ctx context.Context) error { return nil }

func (s *Shapes) Lookup(id string) (Value, Meta, bool) { return Value{}, Meta{}, false }

func (s *Shapes) Pair(ctx context.Context) (int, Value, error) { return 0, Value{}, nil }

func (s *Shapes) Record(ctx context.Context, a, b, c string) error { return nil }

func (s *Shapes) GetWithMeta(ctx context.Context, id string) (Value, Meta, error) {
	return Value{}, Meta{}, nil
}

func (s *Shapes) Mutate(v *Value) {}

func (s *Shapes) Ref(ctx context.Context, id string) *Value { return nil }

func (s *Shapes) Err() error { return nil }

func (s *Shapes) Ready() bool { return false }

func (s *Shapes) Get(ctx context.Context, id string) (Value, error) { return Value{}, nil }

func (s *Shapes) Peek(ctx context.Context, id string) Value { return Value{} }

func (s *Shapes) Find(id string) (Value, bool) { return Value{}, false }

func (s *Shapes) Consume(ctx context.Context, src Source) (int, error) { return 0, nil }

func (s *Shapes) All(ctx context.Context) iter.Seq[Value] { return nil }

func (s *Shapes) Reset() {}

func (s *Shapes) Save(ctx context.Context, v Value) error { return nil }
