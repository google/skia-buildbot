// Package hiddenstore handles storing and retrieving the 'hidden' status of any URL for a given search value.
package hiddenstore

import (
	"context"
	"fmt"
	"net/url"
	"os/user"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/spf13/viper"
	"go.skia.org/infra/go/baseapp"
	skfirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// HiddenStore stores whether a URL found in a source.Artifact should be hidden.
//
// For each hashtag/email we want to keep a list of URLs that should be hidden.
// The URLs correspond to the URLs in source.Artifact. We will only store
// information on URLs that have been hidden, along with the id of the user that
// marked the URL as hidden.
//
// For each hidden URL we will write a document at:
//
//    /hashtag/[instance - skia]/hidden/[hashtag/email-url]
//
// I.e. in a collection named 'hidden' we'll write a document with an id of the
// hashtag/email and the url combined. That document will contain just the URL
// and the hashtag/email. This will allow querying for a specific hashtag/email
// across all documents in the 'hidden' collection.
//
//    {
//      Value: foo
//      URL:  https://.....
//    }
//
// Note that we never store the Artifact type, so this will work no matter what
// set of artifacts we are displaying, and for each hashtag/email query we only
// do a single query of .Where("Value", "==", some-hashtag-or-email).
type HiddenStore struct {
	client           *skfirestore.Client
	hiddenCollection *firestore.CollectionRef
}

// timeout for firestore actions.
const perAttemptTimeout = 5 * time.Second

// hidden is the format of the documents we store in firestore.
type hidden struct {
	URL   string
	Value string
}

// getInstanceName returns the instance name we are to use for firestore.
//
// When running locally the instance name will be $USER, to avoid conflicting
// with production data.
func getInstanceName() string {
	if *baseapp.Local {
		u, err := user.Current()
		if err != nil {
			return "localhost"
		}
		return u.Username
	}
	return viper.GetString("firestore.instance")
}

// New creates a new HiddenStore.
func New() (*HiddenStore, error) {
	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply a
	// token source.
	firestoreClient, err := skfirestore.NewClient(context.Background(), skfirestore.FIRESTORE_PROJECT, "hashtag", getInstanceName(), nil)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &HiddenStore{
		client:           firestoreClient,
		hiddenCollection: firestoreClient.Collection("hidden"),
	}, nil
}

// SetHidden changes the hidden status of the Artifact URL urlValue for the
// given searchValue.
func (h *HiddenStore) SetHidden(ctx context.Context, searchValue, urlValue string, isHidden bool) error {
	// url.QueryEscape escapes slashes in the Doc id, which confuse firestore.
	doc := h.hiddenCollection.Doc(url.QueryEscape(fmt.Sprintf("%s - %s", searchValue, urlValue)))
	var err error
	if !isHidden {
		_, err = h.client.Delete(ctx, doc, 2, perAttemptTimeout)
	} else {
		_, err = h.client.Set(ctx, doc, hidden{
			URL:   urlValue,
			Value: searchValue,
		}, 2, perAttemptTimeout)
	}
	return err
}

// GetHidden returns a slice of all URLs that are hidden for a given searchValue.
func (h *HiddenStore) GetHidden(ctx context.Context, searchValue string) []string {
	ret := []string{}
	q := h.hiddenCollection.Where("Value", "==", searchValue)
	var value hidden
	err := h.client.IterDocs(ctx, "GetHidden", searchValue, q, 2, perAttemptTimeout, func(doc *firestore.DocumentSnapshot) error {
		if err := doc.DataTo(&value); err != nil {
			sklog.Error(err)
			return nil
		}
		ret = append(ret, value.URL)
		return nil
	})
	if err != nil {
		sklog.Errorf("GetHidden: %s", err)
	}
	return ret
}
