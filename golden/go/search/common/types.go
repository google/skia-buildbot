package common

import "go.skia.org/infra/golden/go/sql/schema"

// GroupingDigestKey provides a struct to use as a key combination of
// a grouping ID and a digest.
type GroupingDigestKey struct {
	GroupingID schema.MD5Hash
	Digest     schema.MD5Hash
}
