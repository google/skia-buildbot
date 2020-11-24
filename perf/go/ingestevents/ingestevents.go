// Package ingestevents is a package with helper functions for ingestion PubSub
// events, the ones that are sent when a file in done ingesting and received by
// a clusterer to trigger regression detection. See
// DESIGN.md#event-driven-alerting.
package ingestevents

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// IngestEvent is the PubSub body that is sent from the ingesters each time
// a new file is ingested.
type IngestEvent struct {
	// TraceIDs is a list of all the unencoded trace ids that appeared in the ingested file.
	TraceIDs []string

	// ParamSet is the unencoded ParamSet summary of TraceIDs.
	ParamSet paramtools.ReadOnlyParamSet

	// Filename of the file ingested.
	Filename string
}

// CreatePubSubBody takes an IngestEvent and returns a byte slice that is a
// gzipp'd JSON encoded version of that event. We gzip the to stay below the
// 10MB limit for PubSub data.
func CreatePubSubBody(body *IngestEvent) ([]byte, error) {
	var buf bytes.Buffer
	err := util.WithGzipWriter(&buf, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(body)
	})
	return buf.Bytes(), skerr.Wrap(err)
}

// DecodePubSubBody decodes an IngestEvent encoded by CreatePubSubBody.
func DecodePubSubBody(b []byte) (*IngestEvent, error) {
	var ret IngestEvent
	buf := bytes.NewBuffer(b)
	r, err := gzip.NewReader(buf)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode gzip'd IngestEvent.")
	}
	if err := json.NewDecoder(r).Decode(&ret); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode JSON IngestEvent.")
	}
	return &ret, nil
}
