package typeinfer

// Storage is a grouping of auxiliary extra storage.
type Storage struct {
	arrays  map[Instance]Elems                 // Array elements.
	maps    map[Instance]map[Instance]Instance // Maps.
	structs map[Instance]Fields                // Struct fields.
}

// NewStorage creates a new storage.
func NewStorage() *Storage {
	return &Storage{
		arrays:  make(map[Instance]Elems),
		maps:    make(map[Instance]map[Instance]Instance),
		structs: make(map[Instance]Fields),
	}
}
