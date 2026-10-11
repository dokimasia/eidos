package catalog

// Sink collects the bytes that the catalog writes.
type Sink struct {
	buf []byte
}

// Write appends p to the sink.
func (s *Sink) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	return len(p), nil
}

// Journal records the changes of the catalog in a file.
//
//+reference:writer
type Journal struct {
	// Path is the path of the file of the journal.
	Path string
}

// Mirror copies the bytes that the catalog writes to a second sink.
//
//-reference:writer
type Mirror struct {
	next Sink
}

// Write writes p to the next sink.
func (m *Mirror) Write(p []byte) (int, error) {
	return m.next.Write(p)
}
