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
	Params   []paramtools.Params
	ParamSet paramtools.ParamSet
}

// CreatePubSubBody takes an IngestEvent and returns a byte slice that is a
// gzipp'd JSON encoded version of that event.
func CreatePubSubBody(body *IngestEvent) ([]byte, error) {
	var buf bytes.Buffer
	err := util.WithGzipWriter(&buf, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(body)
	})
	return buf.Bytes(), err
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
