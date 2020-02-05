package json

import (
	"context"
	"encoding/json"
	"io"

	"go.skia.org/infra/task_driver/go/td"
)

// Decode from the given Reader into the given destination.
func Decode(ctx context.Context, r io.Reader, dest interface{}) error {
	return td.Do(ctx, td.Props("JSON Decode").Infra(), func(ctx context.Context) error {
		return json.NewDecoder(r).Decode(dest)
	})
}

// Encode the given data into the given Writer.
func Encode(ctx context.Context, w io.Writer, data interface{}) error {
	return td.Do(ctx, td.Props("JSON Encode").Infra(), func(ctx context.Context) error {
		return json.NewEncoder(w).Encode(data)
	})
}
