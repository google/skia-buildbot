// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
)

// CurrentSchemaVersion is the current schema version of the LogStream.
// Changes that are not backward-compatible should update this field so
// migration logic and scripts can translate appropriately.
const CurrentSchemaVersion = "1"

// LogStream is the primary datastore model containing information and state of
// an individual log stream.
//
// This structure contains the standard queryable fields, and is the source of
// truth for log stream state. Writes to LogStream should be done via Put, which
// will ensure that the LogStream's related query objects are kept in sync.
//
// This structure has additional datastore fields imposed by the
// PropertyLoadSaver. These fields enable querying against some of the complex
// data types:
//	  - _C breaks the the Prefix and Name fields into positionally-queryable
//	    entries. It is used to build globbing queries.
//
//	    It is composed of entries detailing (T is the path [P]refix or [N]ame):
//	    - TF:n:value ("T" has a component, "value", at index "n").
//	    - TR:n:value ("T" has a component, "value", at reverse-index "n").
//	    - TC:count ("T" has "count" total elements).
//
//	    For example, the path "foo/bar/+/baz" would break into:
//	    ["PF:0:foo", "PF:1:bar", "PR:0:bar", "PR:1:foo", "PC:2", "NF:0:baz",
//	     "NR:0:baz", "NC:1"].
//
//	  - _Tags is a string slice containing:
//	    - KEY=[VALUE] key/value tags.
//	    - KEY key presence tags.
type LogStream struct {
	// ID is the LogStream ID. It is generated from the stream's Prefix/Name
	// fields.
	ID HashID `gae:"$id"`

	// Schema is the datastore schema version for this object. This can be used
	// to facilitate schema migrations.
	//
	// The current schema is currentSchemaVersion.
	Schema string

	// Prefix is this log stream's prefix value. Log streams with the same prefix
	// are logically grouped.
	//
	// This value should not be changed once populated, as it will invalidate the
	// ID.
	Prefix string
	// Name is the unique name of this log stream within the Prefix scope.
	//
	// This value should not be changed once populated, as it will invalidate the
	// ID.
	Name string

	// Created is the time when this stream was created.
	Created time.Time

	// Purged, if true, indicates that this log stream has been marked as purged.
	// Non-administrative queries and requests for this stream will operate as
	// if this entry doesn't exist.
	Purged bool
	// PurgedTime is the time when this stream was purged.
	PurgedTime time.Time `gae:",noindex"`

	// ProtoVersion is the version string of the protobuf, as reported by the
	// Collector (and ultimately self-identified by the Butler).
	ProtoVersion string
	// Descriptor is the binary protobuf data LogStreamDescriptor.
	Descriptor []byte `gae:",noindex"`
	// ContentType is the MIME-style content type string for this stream.
	ContentType string
	// StreamType is the data type of the stream.
	StreamType logpb.StreamType
	// Timestamp is the Descriptor's recorded client-side timestamp.
	Timestamp time.Time

	// Tags is a set of arbitrary key/value tags associated with this stream. Tags
	// can be queried against.
	//
	// The serialization/deserialization is handled manually in order to enable
	// key/value queries.
	Tags TagMap `gae:"-"`

	// extra causes datastore to ignore unrecognized fields and strip them in
	// future writes.
	extra ds.PropertyMap `gae:"-,extra"`

	// noDSValidate is a testing parameter to instruct the LogStream not to
	// validate before reading/writing to datastore. It can be controlled by
	// calling SetDSValidate().
	noDSValidate bool
}

var _ interface {
	ds.PropertyLoadSaver
} = (*LogStream)(nil)

// LogStreamID returns the HashID for a given log stream path.
func LogStreamID(path types.StreamPath) HashID {
	return makeHashID(string(path))
}

// LogPrefix returns a keyed (but not loaded) LogPrefix struct for this
// LogStream's Prefix.
func (s *LogStream) LogPrefix() *LogPrefix {
	return &LogPrefix{ID: s.ID}
}

// PopulateState populates the datastore key fields for the supplied
// LogStreamState, binding them to the current LogStream.
func (s *LogStream) PopulateState(di ds.Interface, lst *LogStreamState) {
	lst.Parent = di.KeyForObj(s)
}

// State returns the LogStreamState keyed for this LogStream.
func (s *LogStream) State(di ds.Interface) *LogStreamState {
	var lst LogStreamState
	s.PopulateState(di, &lst)
	return &lst
}

// Path returns the LogDog path for this log stream.
func (s *LogStream) Path() types.StreamPath {
	return types.StreamName(s.Prefix).Join(types.StreamName(s.Name))
}

// Load implements ds.PropertyLoadSaver.
func (s *LogStream) Load(pmap ds.PropertyMap) error {
	// Handle custom properties. Consume them before using the default
	// PropertyLoadSaver.
	for k, v := range pmap {
		if !strings.HasPrefix(k, "_") {
			continue
		}

		switch k {
		case "_Tags":
			// Load the tag map. Ignore errors.
			tm, _ := tagMapFromProperties(v)
			s.Tags = tm
		}
	}

	if err := ds.GetPLS(s).Load(pmap); err != nil {
		return err
	}

	// Validate the log stream. Don't enforce ID correctness, since
	// datastore hasn't populated that field yet.
	if !s.noDSValidate {
		if err := s.validateImpl(false); err != nil {
			return err
		}
	}
	return nil
}

// Save implements ds.PropertyLoadSaver.
func (s *LogStream) Save(withMeta bool) (ds.PropertyMap, error) {
	if !s.noDSValidate {
		if err := s.validateImpl(true); err != nil {
			return nil, err
		}
	}
	s.Schema = CurrentSchemaVersion

	// Save default struct fields.
	pmap, err := ds.GetPLS(s).Save(withMeta)
	if err != nil {
		return nil, err
	}

	// Encode _Tags.
	pmap["_Tags"], err = s.Tags.toProperties()
	if err != nil {
		return nil, fmt.Errorf("failed to encode tags: %v", err)
	}

	// Generate our path components, "_C".
	pmap["_C"] = generatePathComponents(s.Prefix, s.Name)

	return pmap, nil
}

// Validate evaluates the state and data contents of the LogStream and returns
// an error if it is invalid.
func (s *LogStream) Validate() error {
	return s.validateImpl(true)
}

func (s *LogStream) validateImpl(enforceHashID bool) error {
	if enforceHashID {
		// Make sure our Prefix and Name match the Hash ID.
		if hid := LogStreamID(s.Path()); hid != s.ID {
			return fmt.Errorf("hash IDs don't match (%q != %q)", hid, s.ID)
		}
	}

	if err := types.StreamName(s.Prefix).Validate(); err != nil {
		return fmt.Errorf("invalid prefix: %s", err)
	}
	if err := types.StreamName(s.Name).Validate(); err != nil {
		return fmt.Errorf("invalid name: %s", err)
	}
	if s.ContentType == "" {
		return errors.New("empty content type")
	}
	if s.Created.IsZero() {
		return errors.New("created time is not set")
	}

	switch s.StreamType {
	case logpb.StreamType_TEXT, logpb.StreamType_BINARY, logpb.StreamType_DATAGRAM:
		break

	default:
		return fmt.Errorf("unsupported stream type: %v", s.StreamType)
	}

	for k, v := range s.Tags {
		if err := types.ValidateTag(k, v); err != nil {
			return fmt.Errorf("invalid tag [%s]: %s", k, err)
		}
	}

	// Ensure that our Descriptor can be unmarshalled.
	if _, err := s.DescriptorValue(); err != nil {
		return fmt.Errorf("could not unmarshal descriptor: %v", err)
	}
	return nil
}

// DescriptorValue returns the unmarshalled Descriptor field protobuf.
func (s *LogStream) DescriptorValue() (*logpb.LogStreamDescriptor, error) {
	pb := logpb.LogStreamDescriptor{}
	if err := proto.Unmarshal(s.Descriptor, &pb); err != nil {
		return nil, err
	}
	return &pb, nil
}

// LoadDescriptor loads the fields in the log stream descriptor into this
// LogStream entry. These fields are:
//   - Prefix
//   - Name
//   - ContentType
//   - StreamType
//   - Descriptor
//   - Timestamp
//   - Tags
func (s *LogStream) LoadDescriptor(desc *logpb.LogStreamDescriptor) error {
	if err := desc.Validate(true); err != nil {
		return fmt.Errorf("invalid descriptor: %v", err)
	}

	pb, err := proto.Marshal(desc)
	if err != nil {
		return fmt.Errorf("failed to marshal descriptor: %v", err)
	}

	s.Prefix = desc.Prefix
	s.Name = desc.Name
	s.ContentType = desc.ContentType
	s.StreamType = desc.StreamType
	s.Descriptor = pb

	// We know that the timestamp is valid b/c it's checked in ValidateDescriptor.
	if ts := desc.Timestamp; ts != nil {
		s.Timestamp = ds.RoundTime(ts.Time().UTC())
	}

	// Note: tag content was validated via ValidateDescriptor.
	s.Tags = TagMap(desc.Tags)
	return nil
}

// DescriptorProto unmarshals a LogStreamDescriptor from the stream's Descriptor
// field. It will return an error if the unmarshalling fails.
func (s *LogStream) DescriptorProto() (*logpb.LogStreamDescriptor, error) {
	desc := logpb.LogStreamDescriptor{}
	if err := proto.Unmarshal(s.Descriptor, &desc); err != nil {
		return nil, err
	}
	return &desc, nil
}

// SetDSValidate controls whether this LogStream is validated prior to being
// read from or written to datastore.
//
// This is a testing parameter, and should NOT be used in production code.
func (s *LogStream) SetDSValidate(v bool) {
	s.noDSValidate = !v
}

// generatePathComponents generates the "_C" property path components for path
// glob querying.
//
// See the comment on LogStream for more infromation.
func generatePathComponents(prefix, name string) []ds.Property {
	ps, ns := types.StreamName(prefix).Segments(), types.StreamName(name).Segments()

	// Allocate our components array. For each component, there are two entries
	// (forward and reverse), as well as one count entry per component type.
	c := make([]ds.Property, 0, (len(ps)+len(ns)+1)*2)

	gen := func(b string, segs []string) {
		// Generate count component (PC:4).
		c = append(c, ds.MkProperty(fmt.Sprintf("%sC:%d", b, len(segs))))

		// Generate forward and reverse components.
		for i, s := range segs {
			c = append(c,
				// Forward (PF:0:foo).
				ds.MkProperty(fmt.Sprintf("%sF:%d:%s", b, i, s)),
				// Reverse (PR:3:foo)
				ds.MkProperty(fmt.Sprintf("%sR:%d:%s", b, len(segs)-i-1, s)),
			)
		}
	}
	gen("P", ps)
	gen("N", ns)
	return c
}

func addComponentFilter(q *ds.Query, full, f, base string, value types.StreamName) (*ds.Query, error) {
	segments := value.Segments()
	if len(segments) == 0 {
		// All-component query; impose no constraints.
		return q, nil
	}

	// Profile the string. If it doesn't have any glob characters in it,
	// fully constrain the base field.
	hasGlob := false
	for i, seg := range segments {
		switch seg {
		case "*", "**":
			hasGlob = true

		default:
			// Regular segment. Assert that it is valid.
			if err := types.StreamName(seg).Validate(); err != nil {
				return nil, fmt.Errorf("invalid %s component at index %d (%s): %s", full, i, seg, err)
			}
		}
	}
	if !hasGlob {
		// Direct field (full) query.
		return q.Eq(full, string(value)), nil
	}

	// Add specific field constraints for each non-glob segment.
	greedy := false
	rstack := []string(nil)
	for i, seg := range segments {
		switch seg {
		case "*":
			// Skip asserting this segment.
			if greedy {
				// Add a placeholder (e.g., .../**/a/*/b, placeholder ensures "a" gets
				// position -2 instead of -1.
				//
				// Note that "" can never be a segment, because we validate each
				// non-glob path segment and "" is not a valid stream name component.
				rstack = append(rstack, "")
			}
			continue

		case "**":
			if greedy {
				return nil, fmt.Errorf("cannot have more than one greedy glob")
			}

			// Mark that we're greedy, and skip asserting this segment.
			greedy = true
			continue

		default:
			if greedy {
				// Add this to our reverse stack. We'll query the reverse field from
				// this stack after we know how many elements are ultimately in it.
				rstack = append(rstack, seg)
			} else {
				q = q.Eq(f, fmt.Sprintf("%sF:%d:%s", base, i, seg))
			}
		}
	}

	// Add the reverse stack to pin the elements at the end of the path name
	// (e.g., a/b/**/c/d, stack will be {c, d}, need to map to {r.0=d, r.1=c}.
	for i, seg := range rstack {
		if seg == "" {
			// Placeholder, skip.
			continue
		}
		q = q.Eq(f, fmt.Sprintf("%sR:%d:%s", base, len(rstack)-i-1, seg))
	}

	// If we're not greedy, fix this size of this component.
	if !greedy {
		q = q.Eq(f, fmt.Sprintf("%sC:%d", base, len(segments)))
	}
	return q, nil
}

// AddLogStreamPathFilter constructs a compiled LogStreamPathQuery. It will
// return an error if the supllied query string describes an invalid query.
func AddLogStreamPathFilter(q *ds.Query, path string) (*ds.Query, error) {
	prefix, name := types.StreamPath(path).Split()

	err := error(nil)
	q, err = addComponentFilter(q, "Prefix", "_C", "P", prefix)
	if err != nil {
		return nil, err
	}
	q, err = addComponentFilter(q, "Name", "_C", "N", name)
	if err != nil {
		return nil, err
	}
	return q, nil
}

// AddLogStreamTerminatedFilter returns a derived query that asserts that a log
// stream has been terminated.
func AddLogStreamTerminatedFilter(q *ds.Query, v bool) *ds.Query {
	return q.Eq("_Terminated", v)
}

// AddLogStreamArchivedFilter returns a derived query that asserts that a log
// stream has been archived.
func AddLogStreamArchivedFilter(q *ds.Query, v bool) *ds.Query {
	return q.Eq("_Archived", v)
}

// AddLogStreamPurgedFilter returns a derived query that asserts that a log
// stream has been archived.
func AddLogStreamPurgedFilter(q *ds.Query, v bool) *ds.Query {
	return q.Eq("Purged", v)
}

// AddOlderFilter adds a filter to queries that restricts them to results that
// were created before the supplied time.
func AddOlderFilter(q *ds.Query, t time.Time) *ds.Query {
	return q.Lt("Created", t.UTC()).Order("-Created")
}

// AddNewerFilter adds a filter to queries that restricts them to results that
// were created after the supplied time.
func AddNewerFilter(q *ds.Query, t time.Time) *ds.Query {
	return q.Gt("Created", t.UTC()).Order("-Created")
}
