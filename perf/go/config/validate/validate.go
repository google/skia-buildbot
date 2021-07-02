package validate

import (
	"context"
	"errors"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
	"go.skia.org/infra/go/skerr"

	_ "embed" // For embed functionality.
)

var errSchemaViolation = errors.New("schema violation")

//go:embed instanceConfigSchema.json
var schema []byte

// InstanceConfigBytes returns null if the bytes represent a JSON InstanceConfig
// body that conforms to the schema. If err is not nil then the slice of strings
// will contain a list of schema violations.
func InstanceConfigBytes(ctx context.Context, b []byte) ([]string, error) {
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewBytesLoader(b)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed while validating")
	}
	if len(result.Errors()) > 0 {
		formattedResults := make([]string, len(result.Errors()))
		for i, e := range result.Errors() {
			formattedResults[i] = fmt.Sprintf("%d: %s", i, e.String())
		}
		return formattedResults, errSchemaViolation
	}
	return nil, nil
}
