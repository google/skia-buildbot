package db

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	// Gerrit cherrypicks collection name.
	cherrypickDataCol = "CherrypickData"
)

// FirestoreDB uses Cloud Firestore for storage.
type FirestoreDB struct {
	client *firestore.Client
}

// CherrypickData is the type that will be stored in FirestoreDB.
type CherrypickData struct {
	Created      time.Time `firestore:"created"`
	ChangeNumber int64     `firestore:"change_number"`
}

// New returns an instance of FirestoreDB.
func New(ctx context.Context, ts oauth2.TokenSource, fsNamespace, fsProjectId string) (*FirestoreDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, fsProjectId, "cherrypick-watcher", fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client: fsClient,
	}, nil
}

// GetKey sanitizes the source/target repo+branch with the
// change_num to construct an ID appropriate for Firestore.
// Eg: sourceRepo=skia, sourceBranch=chrome/m100, targetRepo=skia,
// targetBranch=android/next-release and changeNum=12 will return
// "skia-chrome-m100_skia-android-next-release_12".
func GetKey(sourceRepo, sourceBranch, targetRepo, targetBranch string, changeNum int64) string {
	return fmt.Sprintf("%s-%s_%s-%s_%d", sourceRepo, strings.Replace(sourceBranch, "/", "-", -1), targetRepo, strings.Replace(targetBranch, "/", "-", -1), changeNum)
}

// GetFromDB returns a CherrypickData document snapshot from Firestore. If the document is not
// found then (nil, nil) is returned.
func (f *FirestoreDB) GetFromDB(ctx context.Context, key string) (*CherrypickData, error) {
	docRef := f.client.Collection(cherrypickDataCol).Doc(key)
	doc, err := f.client.Get(ctx, docRef, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get %s from DB", key)
	}
	cherrypickData := CherrypickData{}
	if err := doc.DataTo(&cherrypickData); err != nil {
		return nil, err
	}
	return &cherrypickData, nil
}

// PutInDB puts the specified CherrypickData into the DB.
func (f *FirestoreDB) PutInDB(ctx context.Context, key string, changeNum int64) error {
	now := time.Now()
	qd := &CherrypickData{
		Created:      now,
		ChangeNumber: changeNum,
	}

	cherrypickCol := f.client.Collection(cherrypickDataCol)
	_, createErr := f.client.Create(ctx, cherrypickCol.Doc(key), qd, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", key)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}
