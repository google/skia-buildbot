// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"errors"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/luci/luci-go/common/runtime/tracer"
)

// Flags contains values parsed from command line arguments.
type Flags struct {
	Quiet     bool
	Verbose   bool
	TracePath string
}

// Init registers flags in a given flag set.
func (d *Flags) Init(f *flag.FlagSet) {
	f.BoolVar(&d.Quiet, "quiet", false, "Get less output")
	f.BoolVar(&d.Verbose, "verbose", false, "Get more output")
	f.StringVar(&d.TracePath, "trace", "", "Name of trace file to generate")
}

// Parse applies changes specified by command line flags.
func (d *Flags) Parse() error {
	if !d.Verbose {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	if d.Quiet && d.Verbose {
		return errors.New("can't use both -quiet and -verbose")
	}
	if d.TracePath != "" {
		p, err := filepath.Abs(d.TracePath)
		if err != nil {
			return err
		}
		d.TracePath = p
	}
	return nil
}

// StartTracing enables tracing and returns a closer that must be called on
// process termination.
func (d *Flags) StartTracing() (io.Closer, error) {
	if d.TracePath == "" {
		return &tracingState{}, nil
	}
	f, err := os.Create(d.TracePath)
	if err != nil {
		return &tracingState{}, err
	}
	if err = tracer.Start(f, 0); err != nil {
		_ = f.Close()
		return &tracingState{}, err
	}
	return &tracingState{f}, nil
}

// Private stuff.

type tracingState struct {
	c io.Closer
}

func (t *tracingState) Close() error {
	if t.c != nil {
		tracer.Stop()
		err := t.c.Close()
		t.c = nil
		return err
	}
	return nil
}
