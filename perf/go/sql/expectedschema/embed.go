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
//go:embed schema.json
var FS embed.FS

// Load returns the deserialized schema.Description stored in the schema.json file.
func Load() (schema.Description, error) {
	var ret schema.Description
	b, err := FS.ReadFile("schema.json")
	if err != nil {
		return ret, skerr.Wrap(err)
	}

	err = json.Unmarshal(b, &ret)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	return ret, nil
}
