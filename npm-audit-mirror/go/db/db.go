package db

import (
	"context"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/npm-audit-mirror/go/types"
)

const (
	// For accessing Firestore.
	defaultAttempts  = 3
	getSingleTimeout = 10 * time.Second
	putSingleTimeout = 10 * time.Second

	// NPM audits collection name.
	NpmAuditDataCol = "NpmAuditData"
	// Downloaded packages examiner collection name.
	DownloadedPackagesExaminerCol = "DownloadedPackagesExaminer"
)

// FirestoreDB uses Cloud Firestore for storage and implements the types.NpmDB
// interface.
type FirestoreDB struct {
	client         *firestore.Client
	collectionName string
}

// New returns an instance of FirestoreDB.
func New(ctx context.Context, ts oauth2.TokenSource, fsNamespace, fsProjectId, collectionName string) (types.NpmDB, error) {
	// Instantiate firestore.
	fsClient, err := firestore.NewClient(ctx, fsProjectId, "npm-audit-mirror", fsNamespace, ts)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore")
	}
	return &FirestoreDB{
		client:         fsClient,
		collectionName: collectionName,
	}, nil
}

// GetFromDB gets a NpmAuditData document snapshot from Firestore. If the
// document is not found then (nil, nil) is returned.
func (f *FirestoreDB) GetFromDB(ctx context.Context, key string) (*types.NpmAuditData, error) {
	docRef := f.client.Collection(f.collectionName).Doc(key)
	doc, err := f.client.Get(ctx, docRef, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
		return nil, nil
	}
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get %s from DB", key)
	}
	npmAuditData := types.NpmAuditData{}
	if err := doc.DataTo(&npmAuditData); err != nil {
		return nil, err
	}
	return &npmAuditData, nil
}

// PutInDB puts NpmAuditData into the DB. If the specified key already exists
// then it is updated.
func (f *FirestoreDB) PutInDB(ctx context.Context, key, issueName string, created time.Time) error {
	qd := &types.NpmAuditData{
		Created:   created,
		IssueName: issueName,
	}

	npmAuditCol := f.client.Collection(f.collectionName)
	_, createErr := f.client.Set(ctx, npmAuditCol.Doc(key), qd, defaultAttempts, putSingleTimeout)
	if st, ok := status.FromError(createErr); ok && st.Code() == codes.AlreadyExists {
		return skerr.Wrapf(createErr, "%s already exists in firestore", key)
	}
	if createErr != nil {
		return createErr
	}

	return nil
}
