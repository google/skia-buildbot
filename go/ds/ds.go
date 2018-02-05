// ds is a package for using Google Cloud Datastore.
package ds

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
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
	ISSUE             Kind = "Issue"
	TRYJOB            Kind = "TryJob"
	TRYJOB_RESULT     Kind = "TryJobResult"
	TRYJOB_EXP_CHANGE Kind = "TryJobExpChange"
	TEST_DIGEST_EXP   Kind = "TestDigestExp"

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
		GOLD_SKIA_PROD_NS:      []Kind{ISSUE, TRYJOB, TRYJOB_RESULT, TRYJOB_EXP_CHANGE, TEST_DIGEST_EXP},
		ANDROID_COMPILE_NS:     []Kind{COMPILE_TASK},
		LEASING_SERVER_NS:      []Kind{TASK},
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
