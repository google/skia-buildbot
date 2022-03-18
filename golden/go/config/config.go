package config

import (
	"io"
	"reflect"

	"github.com/flynn/json5"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// The Common struct is a set of configuration values that are the same across all instances.
// Not all instances will use every field in Common, but every field in Common is used in at least
// two instances (otherwise, it can be deferred to the config specific to its only user). Common
// should be embedded in all configs specific to a given instance (aka. "Specific Configs").
// If a field is defined in both Common and a given specific config, there will be problems, so
// don't do that.
type Common struct {
	// One or more code review systems that we support linking to / commenting on, etc. Used also to
	// identify valid CLs when ingesting data.
	CodeReviewSystems []CodeReviewSystem `json:"code_review_systems"`

	// Google Cloud Storage bucket name.
	GCSBucket string `json:"gcs_bucket"`

	// The primary branch of the git repo to track, e.g. "main".
	GitRepoBranch string `json:"git_repo_branch"`

	// The URL to the git repo that this instance tracks.
	GitRepoURL string `json:"git_repo_url"`

	// GCS path, where the known hashes file should be stored. Format: <bucket>/<path>.
	KnownHashesGCSPath string `json:"known_hashes_gcs_path"`

	// Metrics service address (e.g., ':20000')
	PromPort string `json:"prom_port"`

	// Project ID that houses the pubsub topic.
	PubsubProjectID string `json:"pubsub_project_id"`

	// The port to provide a web handler for /healthz and any other web requests.
	ReadyPort string `json:"ready_port"`

	// URL where this app is hosted.
	SiteURL string `json:"site_url"`

	// SQL username, host and port; typically root@localhost:26234 or root@gold-cockroachdb:26234
	SQLConnection string `json:"sql_connection" optional:"true"`

	// SQL Database name; typically the instance id. e.g. 'flutter', 'skia', etc
	SQLDatabaseName string `json:"sql_database" optional:"true"`

	// TracingProportion overrides the per-service default, which is handy for debugging.
	TracingProportion float64 `json:"tracing_proportion" optional:"true"`

	// Number of recent commits to include in the sliding window of data analysis. Also called the
	// tile size.
	WindowSize int `json:"window_size"`

	// If provided (e.g. ":9002"), a port serving performance-related and other debugging RPCS will
	// be opened up. This RPC will not require authentication.
	DebugPort string `json:"debug_port" optional:"true"`

	// If running locally (not in production).
	Local bool `json:"local"`
}

// CodeReviewSystem represents the details needed to interact with a CodeReviewSystem (e.g.
// "gerrit", "github")
type CodeReviewSystem struct {
	// ID is how this CRS will be identified via query arguments and ingestion data. This is arbitrary
	// and can be used to distinguish between and internal and public version (e.g. "gerrit-internal")
	ID string `json:"id"`

	// Specifies the APIs/code needed to interact ("gerrit", "github").
	Flavor string `json:"flavor"`

	// A URL with %s where a CL ID should be placed to complete it.
	URLTemplate string `json:"url_template"`

	// URL of the Gerrit instance (if any) where we retrieve CL metadata.
	GerritURL string `json:"gerrit_url" optional:"true"`

	// Filepath to file containing GitHub token (if this instance needs to talk to GitHub).
	GitHubCredPath string `json:"github_cred_path" optional:"true"`

	// User and repo of GitHub project to connect to (if any), e.g. google/skia
	GitHubRepo string `json:"github_repo" optional:"true"`
}

// LoadFromJSON5 reads the contents of path and tries to decode the JSON5 there into the provided
// struct. The passed in struct pointer is expected to have "json" struct tags for all fields.
// An error will be returned if any non-struct, non-bool field is its zero value *unless* it is
// tagged with `optional:"true"`.
func LoadFromJSON5(dst interface{}, commonConfigPath, specificConfigPath *string) error {
	// Elem() dereferences a pointer or panics.
	rType := reflect.TypeOf(dst).Elem()
	if rType.Kind() != reflect.Struct {
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

	rValue := reflect.Indirect(reflect.ValueOf(dst))
	return checkRequired(rValue)
}

// checkRequired returns an error if any non-struct, non-bool fields of the given value have a zero
// value *unless* they have an optional tag with value true.
func checkRequired(rValue reflect.Value) error {
	rType := rValue.Type()
	for i := 0; i < rValue.NumField(); i++ {
		field := rType.Field(i)
		if field.Type.Kind() == reflect.Struct {
			if err := checkRequired(rValue.Field(i)); err != nil {
				return err
			}
			continue
		}
		if field.Type.Kind() == reflect.Bool {
			// For ease of use, booleans aren't compared against their zero value, since that would
			// effectively make them required to be true always.
			continue
		}
		isJSON := field.Tag.Get("json")
		if isJSON == "" {
			// don't validate struct values w/o json tags (e.g. config.Duration.Duration).
			continue
		}
		isOptional := field.Tag.Get("optional")
		if isOptional == "true" {
			continue
		}
		// defaults to being required
		if rValue.Field(i).IsZero() {
			return skerr.Fmt("Required %s to be non-zero", field.Name)
		}
	}
	return nil
}

// Duration allows us to supply a duration as a human readable string.
type Duration = config.Duration
