package main

import (
	"context"
	"flag"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"

	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/expectations/fs_expectationstore"
	"go.skia.org/infra/golden/go/fs_utils"
	"go.skia.org/infra/golden/go/types"
)

const (
	maxOperationTime = 2 * time.Minute
)

func main() {
	var (
		fsProjectID = flag.String("fs_project_id", "skia-firestore", "The project with the firestore instance. Datastore and Firestore can't be in the same project.")
		fsNamespace = flag.String("fs_namespace", "", "Typically the instance id. e.g. 'flutter', 'skia', etc")
	)
	flag.Parse()

	fsClient, err := ifirestore.NewClient(context.Background(), *fsProjectID, "gold", *fsNamespace, nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}

	newExpStore, err := fs_expectationstore.New(context.Background(), fsClient, nil, fs_expectationstore.ReadWrite)
	if err != nil {
		sklog.Fatalf("Unable to initialize fs_expstore: %s", err)
	}

	v1 := v1Impl{client: fsClient}

	exp, err := v1.loadV1ExpectationsSharded()
	if err != nil {
		sklog.Fatalf("loading v1 of expectations : %s", err)
	}

	sklog.Debugf("expectations: %#v", exp)
	delta := expectations.AsDelta(exp)
	err = newExpStore.AddChange(context.Background(), delta, "data-migrator")
	if err != nil {
		sklog.Fatalf("Could not write to new fs_expstore: %s", err)
	}

	sklog.Infof("All %d tests with %d entries migrated", exp.NumTests(), exp.Len())
}

const (
	v1expectationsCollection = "expstore_expectations"
	digestField              = "digest"
	v1IssueField             = "issue"

	v1Shards     = 512
	v1MaxRetries = 3
)

type v1Impl struct {
	client *ifirestore.Client
}

// expectationEntry is the document type stored in the expectationsCollection.
type v1ExpectationEntry struct {
	Grouping types.TestName     `firestore:"grouping"`
	Digest   types.Digest       `firestore:"digest"`
	Label    expectations.Label `firestore:"label"`
	Updated  time.Time          `firestore:"updated"`
	Issue    int64              `firestore:"issue"`
}

func (f *v1Impl) loadV1ExpectationsSharded() (*expectations.Expectations, error) {
	// issue = -1 meant master branch in v1
	issue := int64(-1)
	defer metrics2.FuncTimer().Stop()
	q := f.client.Collection(v1expectationsCollection).Where(v1IssueField, "==", issue)

	es := make([]*expectations.Expectations, v1Shards)
	queries := fs_utils.ShardQueryOnDigest(q, digestField, v1Shards)

	err := f.client.IterDocsInParallel(context.Background(), "loadExpectations", strconv.FormatInt(issue, 10), queries, v1MaxRetries, maxOperationTime, func(i int, doc *firestore.DocumentSnapshot) error {
		if doc == nil {
			return nil
		}
		entry := v1ExpectationEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return skerr.Wrapf(err, "corrupt data in firestore, could not unmarshal entry with id %s", id)
		}
		if es[i] == nil {
			es[i] = &expectations.Expectations{}
		}
		es[i].Set(entry.Grouping, entry.Digest, entry.Label)
		return nil
	})

	if err != nil {
		return nil, skerr.Wrapf(err, "fetching expectations for ChangeList %d", issue)
	}

	e := expectations.Expectations{}
	for _, ne := range es {
		e.MergeExpectations(ne)
	}
	return &e, nil
}
