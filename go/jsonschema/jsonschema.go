package jsonschema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/alecthomas/jsonschema"
	"github.com/xeipuuv/gojsonschema"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// ErrSchemaViolation is returned from Validate if the document doesn't conform
// to the schema.
var ErrSchemaViolation = errors.New("schema violation")

// Validate returns null if the document represents a JSON body that conforms to
// the schema. If err is not nil then the slice of strings will contain a list
// of schema violations.
func Validate(ctx context.Context, document, schema []byte) ([]string, error) {
	schemaLoader := gojsonschema.NewBytesLoader(schema)
	documentLoader := gojsonschema.NewBytesLoader(document)
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed while validating")
	}
	if len(result.Errors()) > 0 {
		formattedResults := make([]string, len(result.Errors()))
		for i, e := range result.Errors() {
			formattedResults[i] = fmt.Sprintf("%d: %s", i, e.String())
		}
		return formattedResults, ErrSchemaViolation
	}
	return nil, nil
}

// GenerateSchema writes the JSON Schema for 'v' into 'filename' and will exit
// via sklog.Fatal if any errors occur. This function is designed for use
// in an app you would run via go generate.
func GenerateSchema(filename string, v interface{}) {
	b, err := json.MarshalIndent(jsonschema.Reflect(v), "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	err = util.WithWriteFile(filename, func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
