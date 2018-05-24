// ds is a package for using Google Cloud Datastore.
package ds

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"golang.org/x/sync/errgroup"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/sklog"
)

type Kind string

// Below are all the Kinds used in all applications. New Kinds should be listed
// here, and they should all have unique values, because when defining indexes
// for Cloud Datastore the index config is per project, not pre-namespace.
// Remember to add them to KindsToBackup if they are to be backed up, and push
// a new version of /ds/go/datastore_backup.
const (
	// Predict
	FAILURES     Kind = "Failures"
	FLAKY_RANGES Kind = "FlakyRanges"

	// Perf
	SHORTCUT   Kind = "Shortcut"
	ACTIVITY   Kind = "Activity"
	REGRESSION Kind = "Regression"
	ALERT      Kind = "Alert"

	// Gold
	ISSUE                  Kind = "Issue"
	TRYJOB                 Kind = "Tryjob"
	TRYJOB_RESULT          Kind = "TryjobResult"
	TRYJOB_EXP_CHANGE      Kind = "TryjobExpChange"
	TEST_DIGEST_EXP        Kind = "TryjobTestDigestExp" // TODO(stephana): Remove after migration to consolidated expectations store
	TRYJOB_TEST_DIGEST_EXP Kind = "TryjobTestDigestExp"
	MASTER_EXP_CHANGE      Kind = "MasterExpChange"
	MASTER_TEST_DIGEST_EXP Kind = "MasterTestDigestExp"
	IGNORE_RULE            Kind = "IgnoreRule"
	HELPER_RECENT_KEYS     Kind = "HelperRecentKeys"

	// Android Compile
	COMPILE_TASK Kind = "CompileTask"

	// Leasing
	TASK Kind = "Task"
)

// Namespaces that are used in production, and thus might be backed up.
const (
	// Perf
	PERF_NS                = "perf"
	PERF_ANDROID_NS        = "perf-android"
	PERF_ANDROID_MASTER_NS = "perf-androidmaster"

	// Gold
	GOLD_SKIA_PROD_NS = "gold-skia-prod"

	// Android Compile
	ANDROID_COMPILE_NS = "android-compile"

	// Leasing
	LEASING_SERVER_NS = "leasing-server"
)

var (
	// KindsToBackup is a map from namespace to the list of Kinds to backup.
	// If this value is changed then remember to push a new version of /ds/go/datastore_backup.
	KindsToBackup = map[string][]Kind{
		PERF_NS:                []Kind{ACTIVITY, ALERT, REGRESSION, SHORTCUT},
		PERF_ANDROID_NS:        []Kind{ACTIVITY, ALERT, REGRESSION, SHORTCUT},
		PERF_ANDROID_MASTER_NS: []Kind{ACTIVITY, ALERT, REGRESSION, SHORTCUT},
		GOLD_SKIA_PROD_NS: []Kind{
			HELPER_RECENT_KEYS,
			IGNORE_RULE,
			ISSUE,
			TRYJOB,
			TRYJOB_RESULT,
			TRYJOB_EXP_CHANGE,
			TEST_DIGEST_EXP,
			TRYJOB_TEST_DIGEST_EXP,
			MASTER_EXP_CHANGE,
			MASTER_TEST_DIGEST_EXP,
		},
		ANDROID_COMPILE_NS: []Kind{COMPILE_TASK},
		LEASING_SERVER_NS:  []Kind{TASK},
	}
)

var (
	// DS is the Cloud Datastore client. Valid after Init() has been called.
	DS *datastore.Client

	// Namespace is the datastore namespace that data will be stored in. Valid after Init() has been called.
	Namespace string
)

// InitWithOpt the Cloud Datastore Client (DS).
//
// project - The project name, i.e. "google.com:skia-buildbots".
// ns      - The datastore namespace to store data into.
// opt     - Options to pass to the client.
func InitWithOpt(project string, ns string, opts ...option.ClientOption) error {
	if ns == "" {
		return sklog.FmtErrorf("Datastore namespace cannot be empty.")
	}

	Namespace = ns
	var err error
	DS, err = datastore.NewClient(context.Background(), project, opts...)
	if err != nil {
		return fmt.Errorf("Failed to initialize Cloud Datastore: %s", err)
	}
	return nil
}

// Init the Cloud Datastore Client (DS).
//
// project - The project name, i.e. "google.com:skia-buildbots".
// ns      - The datastore namespace to store data into.
func Init(project string, ns string) error {
	tok, err := auth.NewDefaultJWTServiceAccountTokenSource("https://www.googleapis.com/auth/datastore")
	if err != nil {
		return err
	}
	return InitWithOpt(project, ns, option.WithTokenSource(tok))
}

// InitForTesting is an init to call when running tests. It doesn't do any
// auth as it is expecting to run against the Cloud Datastore Emulator.
// See https://cloud.google.com/datastore/docs/tools/datastore-emulator
//
// project - The project name, i.e. "google.com:skia-buildbots".
// ns      - The datastore namespace to store data into.
func InitForTesting(project string, ns string, kinds ...Kind) error {
	Namespace = ns
	var err error
	DS, err = datastore.NewClient(context.Background(), project)
	if err != nil {
		return fmt.Errorf("Failed to initialize Cloud Datastore: %s", err)
	}
	return nil
}

func Count(client *datastore.Client, kind Kind) (int, error) {
	return client.Count(context.TODO(), NewQuery(kind))
}

func DeleteAll(client *datastore.Client, kind Kind, wait bool) (int, error) {
	overallCount := -1
	for {
		// We choose 500 as the page size, because that's the maximum
		// number of keys that can be deleted in one call.
		sliceIter := newKeySliceIterator(client, kind, 500)
		slice, done, err := sliceIter.next()
		ctx := context.TODO()
		keySlices := [][]*datastore.Key{}

		totalKeyCount := 0
		for !done && (err == nil) {
			keySlices = append(keySlices, slice)
			totalKeyCount += len(slice)
			slice, done, err = sliceIter.next()
		}
		if err != nil {
			return 0, err
		}

		// Delete all slices in parallel.
		var egroup errgroup.Group
		for _, slice := range keySlices {
			func(slice []*datastore.Key) {
				egroup.Go(func() error {
					return client.DeleteMulti(ctx, slice)
				})
			}(slice)
		}
		if err := egroup.Wait(); err != nil {
			return 0, sklog.FmtErrorf("Error deleting entities: %s", err)
		}

		if overallCount == -1 {
			overallCount = totalKeyCount
		}

		// If we need to wait loop until the entity count goes to zero.
		found := 0
		if wait {
			time.Sleep(10 * time.Second)
			if found, err = Count(client, kind); err != nil {
				return 0, err
			}
		}
		if found == 0 {
			break
		}
	}
	return overallCount, nil
}

// Creates a new indeterminate key of the given kind.
func NewKey(kind Kind) *datastore.Key {
	return &datastore.Key{
		Kind:      string(kind),
		Namespace: Namespace,
	}
}

func NewKeyWithParent(kind Kind, parent *datastore.Key) *datastore.Key {
	ret := NewKey(kind)
	ret.Parent = parent
	return ret
}

// Creates a new query of the given kind with the right namespace.
func NewQuery(kind Kind) *datastore.Query {
	return datastore.NewQuery(string(kind)).Namespace(Namespace)
}

func IterKind(client *datastore.Client, kind Kind, instance interface{}, orderedBy ...string) (<-chan interface{}, error) {
	// TODO: set the right page size to get the maximum number of keys at once.
	pageSize := 10

	// Get the type information about the target type
	targetType := reflect.TypeOf(instance)
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	typeKind := targetType.Kind()
	stripPtr := (typeKind == reflect.Slice) || (typeKind == reflect.Map)

	// Get the first slice of keys.
	sliceIter := newKeySliceIterator(client, kind, pageSize, orderedBy...)
	keySlice, done, err := sliceIter.next()
	if err != nil {
		return nil, err
	}

	retCh := make(chan interface{})
	go func() {
		// Close the channel when we are done
		defer close(retCh)

		var err error
		keyCount := 0
		ctx := context.TODO()

		// Process keys until there are no more
		for !done {
			keyCount += len(keySlice)
			sklog.Infof("LOOP: Retrieved %d keys.   Total: %d", len(keySlice), keyCount)

			for _, key := range keySlice {
				// Create a new instance, load it and write it to the output channel
				loadedVal := reflect.New(targetType).Interface()
				if err := client.Get(ctx, key, loadedVal); err != nil {
					sklog.Errorf("Error loading entity with key %v: %s", key, err)
					continue
				}

				// Strip the pointer for slices and maps.
				if stripPtr {
					loadedVal = reflect.ValueOf(loadedVal).Elem().Interface()
				}
				retCh <- loadedVal
			}

			// Get the next slice of keys.
			keySlice, done, err = sliceIter.next()
			if err != nil {
				sklog.Errorf("Error retrieving next key slice: %s", err)
				return
			}
		}
	}()

	return retCh, nil
}

type keySliceIterator struct {
	client    *datastore.Client
	kind      Kind
	pageSize  int
	orderedBy []string
	cursorStr string
	done      bool
}

func newKeySliceIterator(client *datastore.Client, kind Kind, pageSize int, orderedBy ...string) *keySliceIterator {
	return &keySliceIterator{
		client:    client,
		kind:      kind,
		pageSize:  pageSize,
		orderedBy: orderedBy,
		cursorStr: "",
	}
}

func (k *keySliceIterator) next() ([]*datastore.Key, bool, error) {
	// Once we have reached the end, don't run the query again.
	if k.done {
		return nil, true, nil
	}

	query := NewQuery(k.kind).KeysOnly().Limit(k.pageSize)
	for _, ob := range k.orderedBy {
		query = query.Order(ob)
	}

	if k.cursorStr != "" {
		cursor, err := datastore.DecodeCursor(k.cursorStr)
		if err != nil {
			return nil, false, sklog.FmtErrorf("Bad cursor %s: %s", k.cursorStr, err)
		}
		query = query.Start(cursor)
	}

	it := k.client.Run(context.TODO(), query)
	var err error
	var key *datastore.Key
	retKeys := make([]*datastore.Key, 0, k.pageSize)

	for {
		if key, err = it.Next(nil); err != nil {
			break
		}
		retKeys = append(retKeys, key)
	}

	if err != iterator.Done {
		return nil, false, sklog.FmtErrorf("Error retrieving keys: %s", err)
	}

	// Get the string for the next page.
	cursor, err := it.Cursor()
	if err != nil {
		return nil, false, sklog.FmtErrorf("Error retrieving next cursor: %s", err)
	}

	// Check if the string representation of the cursor has changed.
	newCursorStr := cursor.String()
	k.done = (k.cursorStr == newCursorStr)
	k.cursorStr = newCursorStr

	// We are not officially done while we have results to return.
	return retKeys, !(len(retKeys) > 0), nil
}
