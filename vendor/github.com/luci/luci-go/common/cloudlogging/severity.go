// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlogging

import (
	"errors"
	"strings"
)

// Severity defines how important a message is.
type Severity string

// All severity types understood by Cloud Logging.
const (
	Default   Severity = "DEFAULT"
	Debug     Severity = "DEBUG"
	Info      Severity = "INFO"
	Notice    Severity = "NOTICE"
	Warning   Severity = "WARNING"
	Error     Severity = "ERROR"
	Critical  Severity = "CRITICAL"
	Alert     Severity = "ALERT"
	Emergency Severity = "EMERGENCY"
)

var knownSeverity = []Severity{
	Default,
	Debug,
	Info,
	Notice,
	Warning,
	Error,
	Critical,
	Alert,
	Emergency,
}

// Validate returns a error if the severity is not recognized.
func (sev Severity) Validate() error {
	for _, k := range knownSeverity {
		if sev == k {
			return nil
		}
	}
	return errors.New("cloudlog: unknown severity")
}

// String is used by 'flag' package.
func (sev Severity) String() string {
	return string(sev)
}

// Set is called by 'flag' package when parsing command line options.
func (sev *Severity) Set(value string) error {
	val := Severity(strings.ToUpper(value))
	if err := val.Validate(); err != nil {
		return err
	}
	*sev = val
	return nil
}
