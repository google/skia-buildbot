package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	firestore_api "cloud.google.com/go/firestore"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
)

const (
	// For accessing Firestore.
	defaultAttempts  = 3
	getSingleTimeout = 10 * time.Second
	putSingleTimeout = 10 * time.Second

	// PR data collection name.
	pullRequestDataCol = "PullRequestData"
)

// FirestoreDB uses Cloud Firestore for storage.
type FirestoreDB struct {
	client *firestore.Client
	// mtx to control access to firestore
	mtx sync.RWMutex
}

// PullRequestData is the type that will be stored in FirestoreDB.
type PullRequestData struct {
	Created           time.Time `firestore:"created"`
	PullRequestNumber int       `firestore:"pull_request_number"`
	PullRequestURL    string    `firestore:"pull_request_url"`

	Commented bool `firestore:"commented"`
	Merged    bool `firestore:"merged"`
	Abandoned bool `firestore:"abandoned"`
}

// New returns an instance of FirestoreDB.
func New(ctx context.Context, ts oauth2.TokenSource, fsNamespace, fsProjectId string) (*FirestoreDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, fsProjectId, "codereview-watcher", fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client: fsClient,
	}, nil
}

// getPullRequestDocID sanitizes the repo name and constructs an ID appropriate for Firestore.
// Eg: repo=google/skia-buildbot and prNumber=12 returns "google_skia-buildbot_12".
func (f *FirestoreDB) getPullRequestDocID(repo string, prNumber int) string {
	return fmt.Sprintf("%s_%d", strings.Replace(repo, "/", "_", -1), prNumber)
}

// GetFromDB returns a PullRequestData document snapshot from Firestore. If the document is not
// found then (nil, nil) is returned.
func (f *FirestoreDB) GetFromDB(ctx context.Context, repo string, prNumber int) (*PullRequestData, error) {
	f.mtx.RLock()
	defer f.mtx.RUnlock()
	docID := f.getPullRequestDocID(repo, prNumber)
	docRef := f.client.Collection(pullRequestDataCol).Doc(docID)
	doc, err := f.client.Get(ctx, docRef, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get %s from DB", docID)
	}
	prData := PullRequestData{}
	if err := doc.DataTo(&prData); err != nil {
		return nil, err
	}
	return &prData, nil
}

// UpdateDB updates the PullRequestData document corresponding to the repo and prNumber.
func (f *FirestoreDB) UpdateDB(ctx context.Context, repo string, prNumber int, merged, abandoned bool) error {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	docID := f.getPullRequestDocID(repo, prNumber)
	docRef := f.client.Collection(pullRequestDataCol).Doc(docID)
	updates := []firestore_api.Update{{Path: "merged", Value: merged}, {Path: "abandoned", Value: abandoned}}
	if _, err := f.client.Update(ctx, docRef, defaultAttempts, putSingleTimeout, updates); err != nil {
		return skerr.Wrapf(err, "could not update %s in DB", docID)
	}

	return nil
}

// PutInDB puts the specified pull request data into the DB.
func (f *FirestoreDB) PutInDB(ctx context.Context, repo string, prNumber int, prURL string, commented bool) error {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	now := time.Now()
	qd := &PullRequestData{
		Created:           now,
		PullRequestNumber: prNumber,
		PullRequestURL:    prURL,
		Commented:         commented,
	}

	pullRequestCol := f.client.Collection(pullRequestDataCol)
	docID := f.getPullRequestDocID(repo, prNumber)
	_, createErr := f.client.Create(ctx, pullRequestCol.Doc(docID), qd, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", docID)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}
