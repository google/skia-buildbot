package validate

import (
	"context"
	"encoding/json"

	"github.com/qri-io/jsonschema"
	"go.skia.org/infra/go/skerr"

	_ "embed" // For embed functionality.
)

//go:embed instanceConfigSchema.json
var schema []byte

// InstanceConfigBytes returns null if the bytes represent a JSON InstanceConfig
// body that conforms to the schema.
func InstanceConfigBytes(ctx context.Context, b []byte) error {
	rs := &jsonschema.Schema{}
	if err := json.Unmarshal(schema, rs); err != nil {
		return skerr.Wrapf(err, "failed to unmarshal file")
	}

	_, err := rs.ValidateBytes(ctx, b)
	return err
}
