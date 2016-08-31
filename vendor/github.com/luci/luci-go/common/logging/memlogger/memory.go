// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package memlogger

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/logging"
)

// LogEntry is a single entry in a MemLogger, containing a message and a
// severity.
type LogEntry struct {
	Level     logging.Level
	Msg       string
	Data      map[string]interface{}
	CallDepth int
}

// MemLogger is an implementation of Logger.
// Zero value is a valid logger.
type MemLogger struct {
	lock   *sync.Mutex
	data   *[]LogEntry
	fields map[string]interface{}
}

var _ logging.Logger = (*MemLogger)(nil)

// Debugf implements the logging.Logger interface.
func (m *MemLogger) Debugf(format string, args ...interface{}) {
	m.LogCall(logging.Debug, 1, format, args)
}

// Infof implements the logging.Logger interface.
func (m *MemLogger) Infof(format string, args ...interface{}) {
	m.LogCall(logging.Info, 1, format, args)
}

// Warningf implements the logging.Logger interface.
func (m *MemLogger) Warningf(format string, args ...interface{}) {
	m.LogCall(logging.Warning, 1, format, args)
}

// Errorf implements the logging.Logger interface.
func (m *MemLogger) Errorf(format string, args ...interface{}) {
	m.LogCall(logging.Error, 1, format, args)
}

// LogCall implements the logging.Logger interface.
func (m *MemLogger) LogCall(lvl logging.Level, calldepth int, format string, args []interface{}) {
	if m.lock != nil {
		m.lock.Lock()
		defer m.lock.Unlock()
	}
	if m.data == nil {
		m.data = new([]LogEntry)
	}
	*m.data = append(*m.data, LogEntry{lvl, fmt.Sprintf(format, args...), m.fields, calldepth + 1})
}

// Messages returns all of the log messages that this memory logger has
// recorded.
func (m *MemLogger) Messages() []LogEntry {
	if m.lock != nil {
		m.lock.Lock()
		defer m.lock.Unlock()
	}
	if m.data == nil || len(*m.data) == 0 {
		return nil
	}
	ret := make([]LogEntry, len(*m.data))
	copy(ret, *m.data)
	return ret
}

// Reset resets the logged messages recorded so far.
func (m *MemLogger) Reset() {
	if m.lock != nil {
		m.lock.Lock()
		defer m.lock.Unlock()
	}
	if m.data != nil {
		*m.data = nil
	}
}

// GetFunc returns the first matching log entry.
func (m *MemLogger) GetFunc(f func(*LogEntry) bool) *LogEntry {
	if m.lock != nil {
		m.lock.Lock()
		defer m.lock.Unlock()
	}
	if m.data == nil {
		return nil
	}
	for _, ent := range *m.data {
		if f(&ent) {
			clone := ent
			return &clone
		}
	}
	return nil
}

// Get returns the first matching log entry.
// Note that lvl, msg and data have to match the entry precisely.
func (m *MemLogger) Get(lvl logging.Level, msg string, data map[string]interface{}) *LogEntry {
	return m.GetFunc(func(ent *LogEntry) bool {
		return ent.Level == lvl && ent.Msg == msg && reflect.DeepEqual(data, ent.Data)
	})
}

// HasFunc returns true iff the MemLogger contains a matching log message.
func (m *MemLogger) HasFunc(f func(*LogEntry) bool) bool {
	return m.GetFunc(f) != nil
}

// Has returns true iff the MemLogger contains the specified log message.
// Note that lvl, msg and data have to match the entry precisely.
func (m *MemLogger) Has(lvl logging.Level, msg string, data map[string]interface{}) bool {
	return m.Get(lvl, msg, data) != nil
}

// Dump dumps the current memory logger contents to the given writer in a
// human-readable format.
func (m *MemLogger) Dump(w io.Writer) (n int, err error) {
	amt := 0
	for i, msg := range m.Messages() {
		if i == 0 {
			amt, err = fmt.Fprintf(w, "\nDUMP LOG:\n")
			n += amt
			if err != nil {
				return
			}
		}
		if msg.Data == nil {
			amt, err = fmt.Fprintf(w, "  %s: %s\n", msg.Level, msg.Msg)
			n += amt
			if err != nil {
				return
			}
		} else {
			amt, err = fmt.Fprintf(w, "  %s: %s: %s\n", msg.Level, msg.Msg, logging.Fields(msg.Data))
			n += amt
			if err != nil {
				return
			}
		}
	}
	return
}

// Use adds a memory backed Logger to Context, with concrete type
// *MemLogger. Casting to the concrete type can be used to inspect the
// log output after running a test case, for example.
func Use(c context.Context) context.Context {
	lock := sync.Mutex{}
	data := []LogEntry{}
	return logging.SetFactory(c, func(ic context.Context) logging.Logger {
		return &MemLogger{&lock, &data, logging.GetFields(ic)}
	})
}

// Reset is a convenience function to reset the current memory logger.
//
// This will panic if the current logger is not a memory logger.
func Reset(c context.Context) {
	logging.Get(c).(*MemLogger).Reset()
}

// Dump is a convenience function to dump the current contents of the memory
// logger to the writer.
//
// This will panic if the current logger is not a memory logger.
func Dump(c context.Context, w io.Writer) (n int, err error) {
	return logging.Get(c).(*MemLogger).Dump(w)
}

// MustDumpStdout is a convenience function to dump the current contents of the
// memory logger to stdout.
//
// This will panic if the current logger is not a memory logger.
func MustDumpStdout(c context.Context) {
	_, err := logging.Get(c).(*MemLogger).Dump(os.Stdout)
	if err != nil {
		panic(err)
	}
}
