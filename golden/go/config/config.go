package config

import (
	"io"
	"reflect"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// The Common struct is a set of configuration values that are the same across all instances.
// Not all instances will use every field in Common, but every field in Common is used in at least
// two instances (otherwise, it can be deferred to the config specific to its only user). Common
// should be embedded in all configs specific to a given instance (aka. "Specific Configs").
type Common struct {
	// Firestore Namespace; typically the instance id. e.g. 'flutter', 'skia', etc"
	FirestoreNamespace string `json:"fs_namespace"`

	// The project with the Firestore instance. Datastore and Firestore can't be enabled the same
	// project.
	FirestoreProjectID string `json:"fs_project_id"`

	// GCS path, where the known hashes file should be stored. Format: <bucket>/<path>.
	KnownHashesGCSPath string `json:"known_hashes_gcs_path"`

	// Primary CodeReviewSystem (e.g. 'gerrit', 'github
	PrimaryCRS string `json:"primary_crs"`
}

// LoadFromJSON5 reads the contents of path and tries to decode the JSON5 there into the provided
// struct. The passed in struct pointer is expected to have "json" struct tags for all fields.
// An error will be returned if any non-struct field is its zero value *unless* it is tagged
// with `optional:"true"`.
func LoadFromJSON5(dst interface{}, commonConfigPath, specificConfigPath *string) error {
	// Elem() dereferences a pointer or panics.
	rType := reflect.TypeOf(dst).Elem()
	if rType.Kind() != reflect.Struct {
		return skerr.Fmt("Input must be a pointer to a struct, got %T", dst)
	}
	err := util.WithReadFile(*commonConfigPath, func(r io.Reader) error {
		return json5.NewDecoder(r).Decode(&dst)
	})
	if err != nil {
		return skerr.Wrapf(err, "reading common config at %s", *commonConfigPath)
	}
	err = util.WithReadFile(*specificConfigPath, func(r io.Reader) error {
		return json5.NewDecoder(r).Decode(&dst)
	})
	if err != nil {
		return skerr.Wrapf(err, "reading specific config at %s", *specificConfigPath)
	}

	rValue := reflect.Indirect(reflect.ValueOf(dst))
	return checkRequired(rValue)
}

// checkRequired returns an error if any non-struct fields of the given value have a zero value
// *unless* they have an optional tag with value true.
func checkRequired(rValue reflect.Value) error {
	rType := rValue.Type()
	for i := 0; i < rValue.NumField(); i++ {
		field := rType.Field(i)
		if field.Type.Kind() == reflect.Struct {
			if err := checkRequired(rValue.Field(i)); err != nil {
				return err
			}
			continue
		}
		isOptional := field.Tag.Get("optional")
		if isOptional == "true" {
			continue
		}
		// defaults to being required
		if rValue.Field(i).IsZero() {
			return skerr.Fmt("Required %s to be non-zero", field.Name)
		}
	}
	return nil
}
