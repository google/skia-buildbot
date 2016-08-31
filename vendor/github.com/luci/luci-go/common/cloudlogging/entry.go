// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cloudlogging

import (
	"time"

	cloudlog "google.golang.org/api/logging/v1beta3"
)

// Entry is a single log entry. It can be a text message, or a JSONish struct.
type Entry struct {
	// This log's Insert ID, used to uniquely identify this log entry.
	InsertID string
	// Timestamp is an optional timestamp.
	Timestamp time.Time
	// Severity is the severity of the log entry.
	Severity Severity
	// Labels is an optional set of key/value labels for this log entry.
	Labels Labels
	// TextPayload is the log entry payload, represented as a text string.
	TextPayload string
	// StructPayload is the log entry payload, represented as a JSONish structure.
	StructPayload interface{}
}

func (e *Entry) cloudLogEntry(opts *ClientOptions) (*cloudlog.LogEntry, error) {
	entry := cloudlog.LogEntry{
		InsertId: e.InsertID,
		Metadata: &cloudlog.LogEntryMetadata{
			Labels:      e.Labels,
			ProjectId:   opts.ProjectID,
			Region:      opts.Region,
			ServiceName: opts.ServiceName,
			Timestamp:   formatTimestamp(e.Timestamp),
			UserId:      opts.UserID,
			Zone:        opts.Zone,
		},

		TextPayload:   e.TextPayload,
		StructPayload: e.StructPayload,
	}

	// Cloud logging defaults to DEFAULT; therefore, if the entry specifies
	// Default value, there's no need to explicitly include it in the log message.
	if e.Severity != Default {
		if err := e.Severity.Validate(); err != nil {
			return nil, err
		}
		entry.Metadata.Severity = e.Severity.String()
	}

	if !e.Timestamp.IsZero() {
		entry.Metadata.Timestamp = formatTimestamp(e.Timestamp)
	}
	return &entry, nil
}

// formatTimestamp formats a time.Time such that it is compatible with Cloud
// Logging timestamp.
func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
