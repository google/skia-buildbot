// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package streamproto

import (
	"time"

	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
)

// Properties is the set of properties needed to define a LogDog Butler Stream.
type Properties struct {
	// The log stream's descriptor.
	//
	// Note that the Prefix value, if filled, will be overridden by the Butler's
	// Prefix.
	logpb.LogStreamDescriptor

	// Tee is the tee configuration for this stream. If empty, the stream will
	// not be tee'd.
	Tee TeeType

	// Timeout, if specified, is the stream timeout. If a read happens without
	// filling the buffer, it will prematurely return after this period.
	Timeout time.Duration

	// Deadline, if set, specifies the maximum amount of time that data from this
	// Stream can be buffered before being sent to its Output.
	//
	// Note that this value is best-effort, as it is subject to the constraints
	// of the underlying transport medium.
	Deadline time.Duration
}

// Validate validates that the configured Properties are valid and sufficient to
// create a Butler stream.
//
// It skips stream Prefix validation and instead asserts that it is empty, as
// it should not be populated when Properties are defined.
func (p *Properties) Validate() error {
	if err := p.LogStreamDescriptor.Validate(false); err != nil {
		return err
	}
	return nil
}

// Flags is a flag- and JSON-compatible collapse of Properties. It is used
// for stream negotiation protocol and command-line interfaces.
type Flags struct {
	Name                StreamNameFlag `json:"name,omitempty"`
	ContentType         string         `json:"contentType,omitempty"`
	Type                StreamType     `json:"type,omitempty"`
	Timestamp           clockflag.Time `json:"timestamp,omitempty"`
	Tags                TagMap         `json:"tags,omitempty"`
	BinaryFileExtension string         `json:"binaryFileExtension,omitempty"`

	Tee      TeeType            `json:"tee,omitempty"`
	Timeout  clockflag.Duration `json:"timeout,omitempty"`
	Deadline clockflag.Duration `json:"deadline,omitempty"`
}

// Properties converts the Flags to a standard Properties structure.
//
// If the values are not valid, this conversion will return an error.
func (f *Flags) Properties() *Properties {
	contentType := types.ContentType(f.ContentType)
	if contentType == "" {
		contentType = f.Type.DefaultContentType()
	}

	p := &Properties{
		LogStreamDescriptor: logpb.LogStreamDescriptor{
			Name:          string(f.Name),
			ContentType:   string(contentType),
			StreamType:    logpb.StreamType(f.Type),
			Timestamp:     google.NewTimestamp(time.Time(f.Timestamp)),
			BinaryFileExt: f.BinaryFileExtension,
			Tags:          f.Tags,
		},
		Tee:      f.Tee,
		Timeout:  time.Duration(f.Timeout),
		Deadline: time.Duration(f.Deadline),
	}
	return p
}
