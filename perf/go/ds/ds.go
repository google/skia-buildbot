package ds

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/auth"
	"google.golang.org/api/option"
)

var (
	DS        *datastore.Client
	Namespace string
)

func Init(project string, ns string) error {
	Namespace = ns
	tok, err := auth.NewDefaultJWTServiceAccountTokenSource("https://www.googleapis.com/auth/datastore")
	if err != nil {
		return err
	}
	DS, err = datastore.NewClient(context.Background(), "google.com:skia-buildbots", option.WithTokenSource(tok))
	if err != nil {
		return fmt.Errorf("Failed to initialize Cloud Datastore: %s", err)
	}
	return nil
}
