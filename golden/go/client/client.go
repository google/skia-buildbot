// Package client implements high level functions to be used by clients of the
// Gold service, either to use locally or over the network.
package client

import (
	"encoding/json"
	"io"

	"go.skia.org/infra/golden/go/goldingestion"
)

// ValidateIngestionInput validates whether the JSON returned by reading
// from the provided reader, is valid input to the Gold ingestion process.
func ValidateIngestionInput(r io.Reader) error {
	result := &goldingestion.GoldResults{}
	if err := json.NewDecoder(r).Decode(result); err != nil {
		return err
	}
	return nil
}
