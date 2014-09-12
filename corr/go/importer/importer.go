package importer

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Results from the old expections.
type LegacyResults struct {
	ActualResults   map[string]*string         `json:"actual-results"`
	ExpectedResults map[string]*ExpectedResult `json:"expected-results"`
}

// Sub-struct for a single expected result.
type ExpectedResult struct {
	AllowedDigests  []*AllowedDigest `json:"allowed-digests"`
	Bugs            []int            `json:"bugs"`
	ReviewedByHuman *bool            `json:"reviewed-by-human"`
	IgnoreFailure   *bool            `json:"ignore-failure"`
}

// This wraps an array of different types (string, uint64).
// See UnmarshalJSON below
type AllowedDigest struct {
	DigestId string
	Value    uint64
}

// AllowDigest knows how to decode itself from JSON.
func (d *AllowedDigest) UnmarshalJSON(data []byte) error {
	var msg []interface{}

	// Decode into a Number so we don't loose digits (via float64)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	err := dec.Decode(&msg)

	if err == nil {
		d.DigestId = msg[0].(string)
		d.Value, err = strconv.ParseUint(msg[1].(json.Number).String(), 10, 64)
	}
	return err
}

// Decode the whole struct.
func DecodeLegacyResults(jsonData []byte) (*LegacyResults, error) {
	result := new(LegacyResults)
	err := json.Unmarshal(jsonData, result)
	return result, err
}
