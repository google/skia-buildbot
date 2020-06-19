package config

import (
	"io"
	"reflect"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

type Common struct {
	// Firestore Namespace; typically the instance id. e.g. 'flutter', 'skia', etc"
	FirestoreNamespace string `json:"fs_namespace"`
	// The project with the firestore instance. Datastore and Firestore can't be enabled the same
	// project.
	FirestoreProjectID string `json:"fs_project_id"`
	// GCS path, where the known hashes file should be stored.  Format: <bucket>/<path>.
	KnownHashesGCSPath string `json:"known_hashes_gcs_path"`

	// Primary CodeReviewSystem (e.g. 'gerrit', 'github
	PrimaryCRS string `json:"primary_crs"`
}

// LoadFromJSON5 reads the contents of path and tries to decode the JSON there into the provided
// struct.
func LoadFromJSON5(dst interface{}, commonConfigPath, specificConfigPath *string) error {
	// Elem() dereferences a pointer or panics.
	rT := reflect.TypeOf(dst).Elem()
	if rT.Kind() != reflect.Struct {
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

	for i := 0; i < rT.NumField(); i++ {
		field := rT.Field(i)
		isOptional := field.Tag.Get("optional")
		if isOptional == "true" {
			continue
		}
		// defaults to
	}

	// TODO(kjlubick) Make this error if any non-optional values were not set.
	return nil
}
