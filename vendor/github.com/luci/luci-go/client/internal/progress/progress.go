// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package progress

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/luci/luci-go/client/internal/common"
	"github.com/luci/luci-go/common/data/text/units"
)

// Group identifies a column group to keep progress for.
type Group int

// Section identifies a particular column in a column group.
type Section int

// Progress outputs information about the progress of a long task.
//
// It's implementation must be thread safe.
type Progress interface {
	io.Closer
	// Update increases the count of a column.
	Update(group Group, section Section, count int64)
}

// Formatter formats numbers in a Column.
type Formatter func(i int64) string

// Column represent a column to be printed in the progress status.
type Column struct {
	Name      string
	Formatter Formatter
	Value     int64
}

// New returns an initialized thread-safe Progress implementation.
//
// columns is the number of stages each item must go through, then with a set
// of numbers for each state, which will be displayed as a number in each box.
//
// For:
//   columns = [][]Column{{Name:"found"}, {"hashed", Name:"to hash"}}
// It'll print:
//   [found] [hashed/to hash]
func New(columns [][]Column, out io.Writer) Progress {
	p := &progress{
		start:    time.Now().UTC(),
		columns:  make([][]Column, len(columns)),
		interval: time.Second,
		EOL:      "\n",
		out:      out,
	}
	if common.IsTerminal(out) {
		p.interval = 50 * time.Millisecond
		p.EOL = "\r"
	}
	for i, c := range columns {
		p.columns[i] = make([]Column, len(c))
		for j, d := range c {
			p.columns[i][j] = d
			if p.columns[i][j].Formatter == nil {
				p.columns[i][j].Formatter = formatInt
			}
		}
	}
	if out != nil {
		go p.printLoop()
	}
	return p
}

// Private stuff.

type progress struct {
	// Immutable.
	start    time.Time
	interval time.Duration
	EOL      string
	out      io.Writer

	// Mutable.
	lock         sync.Mutex
	columns      [][]Column // Only the .Value are updated.
	valueChanged bool
}

func (p *progress) Update(group Group, section Section, count int64) {
	go func() {
		p.lock.Lock()
		defer p.lock.Unlock()
		p.columns[group][section].Value += count
		p.valueChanged = true
	}()
}

func (p *progress) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.out == nil {
		return errors.New("was already closed")
	}
	_, _ = p.out.Write([]byte("\n"))
	p.out = nil
	return nil
}

func (p *progress) printLoop() {
	line := renderNames(p.columns) + "\n"
	p.lock.Lock()
	out := p.out
	p.lock.Unlock()
	if _, err := io.WriteString(out, line); err != nil {
		return
	}
	for {
		p.lock.Lock()
		out, line := p.printStep()
		p.lock.Unlock()
		if out == nil {
			return
		}
		if line != "" {
			if _, err := io.WriteString(out, line); err != nil {
				return
			}
		}
		time.Sleep(p.interval)
	}
}

func (p *progress) printStep() (io.Writer, string) {
	if p.out == nil || !p.valueChanged {
		return p.out, ""
	}
	p.valueChanged = false
	// Zap resolution at .1s level. We're slow anyway.
	duration := units.Round(time.Since(p.start), 100*time.Millisecond)
	return p.out, fmt.Sprintf("%s %s%s", renderValues(p.columns), duration, p.EOL)
}

func renderSectionName(section []Column) string {
	parts := make([]string, len(section))
	for i, s := range section {
		parts[i] = s.Name
	}
	return "[" + strings.Join(parts, "/") + "]"
}

func renderNames(c [][]Column) string {
	sections := make([]string, len(c))
	for i, section := range c {
		sections[i] = renderSectionName(section)
	}
	return strings.Join(sections, " ")
}

func renderSectionValue(section []Column) string {
	parts := make([]string, len(section))
	for i, part := range section {
		parts[i] = part.Formatter(part.Value)
	}
	return "[" + strings.Join(parts, "/") + "]"
}

func renderValues(c [][]Column) string {
	sections := make([]string, len(c))
	for i, section := range c {
		sections[i] = renderSectionValue(section)
	}
	return strings.Join(sections, " ")
}

func formatInt(i int64) string {
	return strconv.FormatInt(i, 10)
}
