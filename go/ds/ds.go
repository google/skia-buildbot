// ds is a package for using Google Cloud Datastore.
package ds

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"google.golang.org/api/option"
)

type Kind string

var (
	// DS is the Cloud Datastore client. Valid after Init() has been called.
	DS *datastore.Client

	// Namespace is the datastore namespace that data will be stored in. Valid after Init() has been called.
	Namespace string
)

// Init the Cloud Datastore Client (DS).
//
// project - The project name, i.e. "google.com:skia-buildbots".
// ns      - The datastore namespace to store data into.
func Init(project string, ns string) error {
	Namespace = ns
	tok, err := auth.NewDefaultJWTServiceAccountTokenSource("https://www.googleapis.com/auth/datastore")
	if err != nil {
		return err
	}
	DS, err = datastore.NewClient(context.Background(), project, option.WithTokenSource(tok))
	if err != nil {
		return fmt.Errorf("Failed to initialize Cloud Datastore: %s", err)
	}
	return nil
}

// InitForTesting is an init to call when running tests. It doesn't do any
// auth as it is expecting to run against the Cloud Datastore Emulator.
// See https://cloud.google.com/datastore/docs/tools/datastore-emulator
//
// project - The project name, i.e. "google.com:skia-buildbots".
// ns      - The datastore namespace to store data into.
func InitForTesting(project string, ns string) error {
	Namespace = ns
	var err error
	DS, err = datastore.NewClient(context.Background(), project)
	if err != nil {
		return fmt.Errorf("Failed to initialize Cloud Datastore: %s", err)
	}
	return nil
}

// Creates a new indeterminate key of the given kind.
func NewKey(kind Kind) *datastore.Key {
	return &datastore.Key{
		Kind:      string(kind),
		Namespace: Namespace,
	}
}

// Creates a new query of the given kind with the right namespace.
func NewQuery(kind Kind) *datastore.Query {
	return datastore.NewQuery(string(kind)).Namespace(Namespace)
}
