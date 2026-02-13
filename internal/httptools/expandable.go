package httptools

import "encoding/json"

// Expandable represents a field with three JSON serialization states:
//   - Not set (zero value): omitted from JSON via omitzero
//   - Set to null: serialized as JSON null
//   - Set to value: serialized as the value
type Expandable[T any] struct {
	set   bool
	value *T
}

// Set creates an Expandable with a value.
func Set[T any](v T) Expandable[T] {
	return Expandable[T]{set: true, value: &v}
}

// Null creates an Expandable that serializes as JSON null.
func Null[T any]() Expandable[T] {
	return Expandable[T]{set: true, value: nil}
}

// IsZero reports whether the field was not set (for omitzero support).
func (e Expandable[T]) IsZero() bool {
	return !e.set
}

// MarshalJSON implements json.Marshaler.
func (e Expandable[T]) MarshalJSON() ([]byte, error) {
	if e.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*e.value)
}
