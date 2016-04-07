package limitwriter

import "io"

// LimitWriter implements io.Writer and writes the data to an io.Writer, but
// limits the total bytes written to it, dropping the remaining bytes on the
// floor.
type LimitWriter struct {
	dst   io.Writer
	limit int
}

// New create a new LimitWriter that accepts at most 'limit' bytes.
func New(dst io.Writer, limit int) *LimitWriter {
	return &LimitWriter{
		dst:   dst,
		limit: limit,
	}
}

func (l *LimitWriter) Write(p []byte) (int, error) {
	lp := len(p)
	var err error
	if l.limit > 0 {
		if lp > l.limit {
			p = p[:l.limit]
		}
		l.limit -= len(p)
		_, err = l.dst.Write(p)
	}
	return lp, err
}
