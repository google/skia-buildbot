package limitwriter

import "io"

// LimitWriter implements io.Writer and writes the data to an io.Writer, but
// limits the total bytes written to it, dropping the remaining bytes on the
// floor.
type LimitWriter struct {
	dst       io.Writer
	remaining int
}

// New create a new LimitWriter that accepts at most 'limit' bytes.
func New(dst io.Writer, limit int) *LimitWriter {
	return &LimitWriter{
		dst:       dst,
		remaining: limit,
	}
}

func (l *LimitWriter) Write(p []byte) (int, error) {
	lp := len(p)
	var err error
	if l.remaining > 0 {
		if lp > l.remaining {
			p = p[:l.remaining]
		}
		_, err = l.dst.Write(p)
	}
	l.remaining -= lp
	return lp, err
}

// Overrun returns the number of bytes dropped. Returns 0 if all data was
// written.
func (l *LimitWriter) Overrun() int {
	if l.remaining < 0 {
		return -l.remaining
	} else {
		return 0
	}
}
