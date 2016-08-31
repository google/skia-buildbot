// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinatorTest

import (
	"bytes"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// TestSecret returns a testing types.StreamPrefix.
func TestSecret() types.PrefixSecret {
	return types.PrefixSecret(bytes.Repeat([]byte{0x6F}, types.PrefixSecretLength))
}

// TestStream returns a testing stream.
type TestStream struct {
	// Project is the project name for this stream.
	Project config.ProjectName
	// Path is the path of this stream.
	Path types.StreamPath

	// Desc is the log stream descriptor.
	Desc *logpb.LogStreamDescriptor

	// Prefix is the Coordinator LogPrefix entity.
	Prefix *coordinator.LogPrefix
	// Prefix is the Coordinator LogStreamState entity.
	State *coordinator.LogStreamState
	// Prefix is the Coordinator LogStream entity.
	Stream *coordinator.LogStream
}

// MakeStream builds a new TestStream with the supplied parameters.
func MakeStream(c context.Context, project config.ProjectName, path types.StreamPath) *TestStream {
	prefix, name := path.Split()

	now := clock.Now(c).UTC()
	secret := TestSecret()

	ts := TestStream{
		Project: project,
		Prefix: &coordinator.LogPrefix{
			ID:         "", // Filled in by Reload.
			Created:    ds.RoundTime(now),
			Prefix:     "", // Filled in by Reload.
			Source:     []string{"test suite"},
			Expiration: ds.RoundTime(now.Add(24 * time.Hour)),
			Secret:     secret,
		},
		Desc: &logpb.LogStreamDescriptor{
			Prefix:      string(prefix),
			Name:        string(name),
			StreamType:  logpb.StreamType_TEXT,
			ContentType: "application/text",
			Timestamp:   google.NewTimestamp(now),
		},
		State: &coordinator.LogStreamState{
			Parent:        nil, // Filled in by Reload.
			Created:       ds.RoundTime(now),
			Updated:       time.Time{}, // Filled in by Reload.
			Secret:        secret,
			TerminalIndex: -1,
		},
		Stream: &coordinator.LogStream{
			ID:           "", // Filled in by Reload.
			ProtoVersion: logpb.Version,
			Created:      ds.RoundTime(now),
			// Descriptor-derived fields filled in by Reload.
		},
	}
	ts.Reload(c)
	return &ts
}

// Reload loads derived fields from their base fields.
func (ts *TestStream) Reload(c context.Context) {
	ts.Path = ts.Desc.Path()

	// LogPrefix
	ts.Prefix.Prefix = ts.Desc.Prefix
	ts.Prefix.ID = coordinator.LogPrefixID(types.StreamName(ts.Prefix.Prefix))

	// LogStream
	ts.Stream.ID = coordinator.LogStreamID(ts.Path)
	if err := ts.Stream.LoadDescriptor(ts.Desc); err != nil {
		panic(err)
	}

	// LogStreamState
	ts.State.Updated = ds.RoundTime(clock.Now(c)).UTC()
	ts.WithProjectNamespace(c, func(c context.Context) {
		ts.State.Parent = ds.Get(c).KeyForObj(ts.Stream)
	})
}

// DescBytes returns the marshalled descriptor bytes.
func (ts *TestStream) DescBytes() []byte {
	v, err := proto.Marshal(ts.Desc)
	if err != nil {
		panic(err)
	}
	return v
}

// Put adds all of the entities for this TestStream to the datastore.
func (ts *TestStream) Put(c context.Context) (err error) {
	ts.WithProjectNamespace(c, func(c context.Context) {
		err = ds.Get(c).Put(ts.Prefix, ts.State, ts.Stream)
	})
	return
}

// Get reloads all of the entities for this TestStream.
func (ts *TestStream) Get(c context.Context) (err error) {
	ts.WithProjectNamespace(c, func(c context.Context) {
		err = ds.Get(c).Get(ts.Prefix, ts.State, ts.Stream)
	})
	return
}

// LogEntry generates a generic testing log entry for this stream with the
// specific log stream index.
func (ts *TestStream) LogEntry(c context.Context, i int) *logpb.LogEntry {
	le := logpb.LogEntry{
		TimeOffset:  google.NewDuration(clock.Now(c).Sub(ts.Stream.Created)),
		StreamIndex: uint64(i),
	}

	message := fmt.Sprintf("log entry #%d", i)
	switch ts.Desc.StreamType {
	case logpb.StreamType_TEXT:
		le.Content = &logpb.LogEntry_Text{
			&logpb.Text{
				Lines: []*logpb.Text_Line{
					{
						Value:     message,
						Delimiter: "\n",
					},
				},
			},
		}

	case logpb.StreamType_BINARY:
		le.Content = &logpb.LogEntry_Binary{
			&logpb.Binary{
				Data: []byte(message),
			},
		}

	case logpb.StreamType_DATAGRAM:
		le.Content = &logpb.LogEntry_Datagram{
			&logpb.Datagram{
				Data: []byte(message),
			},
		}
	}
	return &le
}

// WithProjectNamespace runs f in proj's namespace, bypassing authentication
// checks.
func (ts *TestStream) WithProjectNamespace(c context.Context, f func(context.Context)) {
	WithProjectNamespace(c, ts.Project, f)
}
