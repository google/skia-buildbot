package common

import "go.skia.org/infra/golden/go/sql/schema"

// GroupingDigestKey provides a struct to use as a key combination of
// a grouping ID and a digest.
type GroupingDigestKey struct {
	GroupingID schema.MD5Hash
	Digest     schema.MD5Hash
}

type DigestWithTraceAndGrouping struct {
	TraceID    schema.TraceID
	GroupingID schema.GroupingID
	Digest     schema.DigestBytes
	// OptionsID will be set for CL data only; for primary data we have to look it up from a
	// different table and the options could change over time.
	OptionsID schema.OptionsID
}

type FilterSets struct {
	Key    string
	Values []string
}
