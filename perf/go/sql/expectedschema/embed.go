// Package expectedschema contains the schema the database is expected to have.
package expectedschema

import (
	"embed" // Enable go:embed.
	"encoding/json"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/config"
)

// FS is a filesystem with the schema.json file.
//
//go:embed schema.json
//go:embed schema_prev.json
//go:embed schema_spanner.json
//go:embed schema_prev_spanner.json
var FS embed.FS

// Load returns the deserialized schema.Description stored in the schema.json file.
func Load(datastoreType config.DataStoreType) (schema.Description, error) {
	var ret schema.Description
	fileName := "schema.json"
	if datastoreType == config.SpannerDataStoreType {
		fileName = "schema_spanner.json"
	}
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
func LoadPrev(datastoreType config.DataStoreType) (schema.Description, error) {
	var ret schema.Description
	fileName := "schema_prev.json"
	if datastoreType == config.SpannerDataStoreType {
		fileName = "schema_prev_spanner.json"
	}
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
