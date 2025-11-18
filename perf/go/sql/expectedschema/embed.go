// Package expectedschema contains the schema the database is expected to have.
package expectedschema

import (
	"embed" // Enable go:embed.
	"encoding/json"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/schema"
)

// FS is a filesystem with the schema.json file.
//
//go:embed schema_spanner.json
//go:embed schema_prev_spanner.json
var FS embed.FS

// Load returns the deserialized schema.Description stored in the schema.json file.
func Load() (schema.Description, error) {
	var ret schema.Description
	fileName := "schema_spanner.json"
	b, err := FS.ReadFile(fileName)
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	err = json.Unmarshal(b, &ret)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	return ret, nil
}

// LoadPrev returns the deserialized schema.Description stored in the schema_old.json file.
func LoadPrev() (schema.Description, error) {
	var ret schema.Description
	fileName := "schema_prev_spanner.json"
	b, err := FS.ReadFile(fileName)
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	err = json.Unmarshal(b, &ret)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	return ret, nil
}
