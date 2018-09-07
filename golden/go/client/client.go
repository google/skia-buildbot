package client

import (
	"encoding/json"
	"io"

	"go.skia.org/infra/golden/go/goldingestion"
)

func ValidateIngestionInput(r io.Reader) error {
	result := &goldingestion.GoldResults{}
	if err := json.NewDecoder(r).Decode(result); err != nil {
		return err
	}
	return nil
}
