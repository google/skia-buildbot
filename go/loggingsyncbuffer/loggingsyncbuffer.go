// Package loggingsyncbuffer contains a SyncWriter that writes to a buffer.
package loggingsyncbuffer

import (
	"bytes"

	"github.com/jcgregorio/logger"
)

// SyncWriter implements logger.SyncWriter.
type SyncWriter struct {
	b *bytes.Buffer
}

// New returns a new SyncWriter.
func New() *SyncWriter {
	return &SyncWriter{
		b: &bytes.Buffer{},
	}
}

// Write implements logger.SyncWriter.
func (f *SyncWriter) Write(p []byte) (n int, err error) {
	return f.b.Write(p)
}

// Sync implements logger.SyncWriter.
func (f *SyncWriter) Sync() error {
	return nil
}

// String returns the contents of the buffer.
func (f *SyncWriter) String() string {
	return f.b.String()
}

// Assert we implement the logger.SyncWriter interface.
var _ logger.SyncWriter = (*SyncWriter)(nil)
