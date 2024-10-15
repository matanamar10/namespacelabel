package set

// Set is a generic data structure that represents a set of unique items.
type Set[T comparable] map[T]struct{}

// NewSet creates a new empty Set.
func NewSet[T comparable]() Set[T] {
	return make(Set[T])
}

// Add adds a value to the Set.
func (s Set[T]) Add(value T) {
	s[value] = struct{}{}
}

// Contains checks if a value exists in the Set.
func (s Set[T]) Contains(value T) bool {
	_, exists := s[value]
	return exists
}
