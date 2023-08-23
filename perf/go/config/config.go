package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"time"

	iSchema "github.com/invopop/jsonschema"
	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/notifytypes"

	_ "embed" // For embed functionality.
)

var errSchemaViolation = errors.New("schema violation")

// schema is a json schema for InstanceConfig, it is created by
// running go generate on ./generate/main.go.
//
//go:embed instanceConfigSchema.json
var schema []byte

const (
	// MaxSampleTracesPerCluster  is the maximum number of traces stored in a
	// ClusterSummary.
	MaxSampleTracesPerCluster = 50

	// MinStdDev is the smallest standard deviation we will normalize, smaller
	// than this and we presume it's a standard deviation of zero.
	MinStdDev = 0.001

	// GotoRange is the number of commits on either side of a target commit we
	// will display when going through the goto redirector.
	GotoRange = 10
)

// AuthConfig provides details how authentication is done, which is by Auth
// Proxy. See, for example,
// https://grafana.com/docs/grafana/latest/auth/auth-proxy/
type AuthConfig struct {
	// HeaderName is the name of the header that contains the logged in users
	// email. E.g. X-WEBAUTH-USER.
	HeaderName string `json:"header_name"`

	// A regex to extract the users email address from the header, in case
	// EmailRegex is a regex to extract the email address from the header value.
	// This value can be empty. This is useful for reverse proxies that include
	// other information in the header in addition to the email address, such as
	// https://cloud.google.com/iap/docs/identity-howto#getting_the_users_identity_with_signed_headers
	//
	// If supplied, the Regex must have a single subexpression that matches the
	// email address.
	EmailRegex string `json:"email_regex,omitempty"`
}

type NotifyConfig struct {
	// Notifications chooses how notifications are sent when a regression is found.
	Notifications notifytypes.Type `json:"notifications"`

	// IssueTrackerAPIKeySecretProject is the name of the GCP project where the
	// issue tracker API key is stored in the secret manager. Only required if
	// Notifications is set to use an issue tracker.
	IssueTrackerAPIKeySecretProject string `json:"issue_tracker_api_key_secret_project,omitempty"`

	// IssueTrackerAPIKeySecretName is the name of the secret in the secret
	// manager that contains the issue tracker API key. Only required if
	// Notifications is set to use an issue tracker.
	IssueTrackerAPIKeySecretName string `json:"issue_tracker_api_key_secret_name,omitempty"`
}

// DataStoreType determines what type of datastore to build. Applies to
// tracestore.Store, alerts.Store, regression.Store, and shortcut.Store.
type DataStoreType string

const (
	// CockroachDBDataStoreType is for storing all data in a CockroachDB database.
	CockroachDBDataStoreType DataStoreType = "cockroachdb"
)

// CacheConfig is the config for LRU caches in the trace store.
type CacheConfig struct {
	// The names of the memcached servers to use, for example:
	//
	//  "memcached_servers": [
	//        "perf-memcached-0.perf-memcached:11211",
	//        "perf-memcached-1.perf-memcached:11211",
	//  ]
	//
	// If the list is empty or nil then memcached will not be used and an
	// in-memory lru cache will be used.
	MemcachedServers []string `json:"memcached_servers"`

	// The name to postfix to keys, to allow more than one instance of Perf to
	// use a common memcached cluster.
	Namespace string `json:"namespace"`
}

// DataStoreConfig is the configuration for how Perf stores data.
type DataStoreConfig struct {
	// DataStoreType determines what type of datastore to build. This value will
	// determine how the rest of the DataStoreConfig values are interpreted.
	DataStoreType DataStoreType `json:"datastore_type"`

	// If the datastore type is 'cockroachdb' then this value is a connection
	// string of the form "postgresql://...". See
	// https://www.cockroachlabs.com/docs/stable/connection-parameters.html for
	// more details.
	//
	// In addition, for 'cockroachdb' databases, the database name given in the
	// connection string must exist and the user given in the connection string
	// must have rights to create, delete, and alter tables as Perf will do
	// database migrations on startup.
	ConnectionString string `json:"connection_string"`

	// TileSize is the size of each tile in commits. This value is used for all
	// datastore types.
	TileSize int32 `json:"tile_size"`

	// CacheConfig is the config for LRU caches in the trace store.
	CacheConfig *CacheConfig `json:"cache,omitempty"`

	// EnableFollowerReads, if true, means older data in the database can be
	// used to respond to queries, which is faster, but is not appropriate if
	// data recency is imperative. The age of the data should only be 5s older.
	EnableFollowerReads bool `json:"enable_follower_reads,omitempty"`
}

// SourceType determines what type of file.Source to build from a SourceConfig.
type SourceType string

const (
	// GCSSourceType is for Google Cloud Storage.
	GCSSourceType SourceType = "gcs"

	// DirSourceType is for a local filesystem directory and is only appropriate
	// for tests and demo mode.
	DirSourceType SourceType = "dir"
)

// SourceConfig is the config for where ingestable files come from.
type SourceConfig struct {
	// SourceType is the type of file.Source to use. This value will determine
	// how the rest of the SourceConfig values are interpreted.
	SourceType SourceType `json:"source_type"`

	// Project is the Google Cloud Project name. Only used for source of type
	// "gcs".
	Project string `json:"project"`

	// Topic is the PubSub topic when new files arrive to be ingested. Only used
	// for source of type "gcs".
	Topic string `json:"topic"`

	// Subscription is the name of the subscription to use when requestion
	// events from the PubSub Topic. If not supplied then a name that
	// incorporates the Topic name will be used.
	Subscription string `json:"subscription"`

	// DeadLetterTopic is the PubSub dead letter topic to use when a message cannot be handled.
	// When this attribute is configed:
	// If the Pub/Sub service attempts to deliver a message
	// but the subscriber can't acknowledge it within the maximum number of delivery attempts,
	// Pub/Sub will forward the undeliverable message to a dead-letter topic
	// Pub/Sub dead letter topic doc:
	// https://cloud.google.com/pubsub/docs/handling-failures#dead_letter_topic
	// Only used for source of type "gcs".
	DeadLetterTopic string `json:"dl_topic,omitempty"`

	// DeadLetterSubscription is the name of the dead letter subscription to use when
	// a message cannot be handled.
	// To avoid losing messages from the dead-letter topic,
	// attach at least one dead-letter subscription to the dead-letter topic.
	// The dead-letter subscription receives messages from the dead-letter topic
	// Pub/Sub dead-letter topic doc:
	// https://cloud.google.com/pubsub/docs/handling-failures#configure_a_dead_letter_topic
	// Only used for source of type "gcs".
	DeadLetterSubscription string `json:"dl_subscription,omitempty"`

	// Sources is the list of sources of data files. For a source of "gcs" this
	// is a list of Google Cloud Storage URLs, e.g.
	// "gs://skia-perf/nano-json-v1". For a source of type "dir" is must only
	// have a single entry and be populated with a local filesystem directory
	// name.
	Sources []string `json:"sources"`

	// RejectIfNameMatches is a regex. If it matches the file.Name then the file
	// will be ignored. Leave the empty string to disable rejection.
	RejectIfNameMatches string `json:"reject_if_name_matches,omitempty"`

	// AcceptIfNameMatches is a regex. If it matches the file.Name the file will
	// be processed. Leave the empty string to accept all files.
	AcceptIfNameMatches string `json:"accept_if_name_matches,omitempty"`
}

// IngestionConfig is the configuration for how source files are ingested into
// being traces in a TraceStore.
type IngestionConfig struct {
	// SourceConfig is the config for where files to ingest come from.
	SourceConfig SourceConfig `json:"source_config"`

	// Branches, if populated then restrict to ingesting just these branches.
	//
	// Only use this if the Subject of each commit in the repo ends with the
	// branch name, otherwise this will break the clustering page.
	Branches []string `json:"branches"`

	// FileIngestionTopicName is the PubSub topic name we should use if doing
	// event driven regression detection. The ingesters use this to know where
	// to emit events to, and the clusterers use this to know where to make a
	// subscription.
	//
	// Should only be turned on for instances that have a huge amount of data,
	// i.e. >500k traces, and that have sparse data.
	//
	// This should really go away, IngestionConfig should be used to build
	// an interface that ingests files and optionally provides a channel
	// of events when a file is ingested.
	FileIngestionTopicName string `json:"file_ingestion_pubsub_topic_name"`
}

// GitAuthType is the type of authentication Git should use, if any.
type GitAuthType string

const (
	// GitAuthNone implies no authentication is needed when cloning/pulling a
	// Git repo, i.e. it is public. The value is the empty string so that the
	// default is no authentication.
	GitAuthNone GitAuthType = ""

	// GitAuthGerrit is for repos that are hosted by Gerrit and require
	// authentication. This setting implies that a
	// GOOGLE_APPLICATION_CREDENTIALS environment variable will be set and the
	// associated service account has read access to the Gerrit repo.
	GitAuthGerrit GitAuthType = "gerrit"
)

// GitProvider is the method used to interrogate git repos.
type GitProvider string

const (
	// GitProviderCLI uses a local copy of git to checkout the repo.
	GitProviderCLI GitProvider = "git"

	// GitProviderGitiles uses the Gitiles API.
	GitProviderGitiles GitProvider = "gitiles"
)

// AllGitProviders is a slice of all valid GitProviders.
var AllGitProviders []GitProvider = []GitProvider{
	GitProviderCLI,
	GitProviderGitiles,
}

// GitRepoConfig is the config for the git repo.
type GitRepoConfig struct {
	// GitAuthType is the type of authentication the repo requires. Defaults to
	// GitAuthNone.
	GitAuthType GitAuthType `json:"git_auth_type,omitempty"`

	// Provider is the method used to interrogate git repos.
	Provider GitProvider `json:"provider"`

	// StartCommit is the commit in the repo where we start tracking commits,
	// i.e. StartCommit will have a Commit Number of 0. If not supplied then
	// default to the first commit in the repo. This is used to avoid having to
	// ingest all the commits in a huge repo where we don't care about the
	// majority of the history, e.g. Chrome.
	StartCommit string `json:"start_commit,omitempty"`

	// URL that the Git repo is fetched from.
	URL string `json:"url"`

	// Dir is the directory into which the repo should be checked out.
	Dir string `json:"dir"`

	// FileChangeMarker is a path in the git repo to watch for changes. If the
	// file indicated changes in a commit then a marker will be displayed on the
	// graph at that commit.
	FileChangeMarker string `json:"file_change_marker,omitempty"`

	// DebouceCommitURL signals if a link to a Git commit needs to be specially
	// dereferenced. That is, some repos are synthetic and just contain a single
	// file that changes, with a commit message that is a URL that points to the
	// true source of information. If this value is true then links to commits
	// need to be debounced and use the commit message instead.
	DebouceCommitURL bool `json:"debounce_commit_url,omitempty"`

	// CommitURL is a Go format string that joins the GitRepoConfig URL with a
	// commit hash to produce the URL of a web page that shows that exact
	// commit. For example "%s/commit/%s" would be a good value for GitHub
	// repos, while "%s/+show/%s" is a good value for Gerrit repos. Defaults
	// to "%s/+show/%s" if no value is supplied.
	CommitURL string `json:"commit_url,omitempty"`

	// CommitNumberRegex is the regex we use to get commit number from the
	// message section of git log.
	// This field also indicates whether the commit number should be used
	// Git log example: "... Cr-Commit-Position: refs/heads/master@{#727901}"
	// Leave empty to have Perf generate commit numbers.
	CommitNumberRegex string `json:"commit_number_regex,omitempty"`
}

// DurationAsString allows serializing a Duration as a string, and also handles
// deserializing the empty string.
type DurationAsString time.Duration

// MarshalJSON implements json.Marshaler.
func (d DurationAsString) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *DurationAsString) UnmarshalJSON(b []byte) error {
	inputAsString := string(b)
	fmt.Println(inputAsString)
	var asString string
	if err := json.Unmarshal(b, &asString); err != nil {
		return skerr.Wrap(err)
	}
	if asString == "" {
		*d = 0
		return nil
	}

	tmp, err := time.ParseDuration(asString)
	if err != nil {
		return skerr.Wrap(err)
	}
	*d = DurationAsString(tmp)
	return nil
}

// JSONSchema defines the JSON Schema that will be generated for
// DurationAsString, forcing it to be a string instead of an int.
func (DurationAsString) JSONSchema() *iSchema.Schema {
	return &iSchema.Schema{
		Type:        "string",
		Title:       "Duration",
		Description: "A golang time.Duration serialized as a string.",
	}
}

// AnomalyConfig contains the settings for Anomaly detection.
type AnomalyConfig struct {
	// SettlingTime is the amount of time to wait before including data from a
	// commit in Anomaly detection.
	//
	// For example, because of machine contention or retries for some tests, the
	// results may arrive out of order causing Anomalies to be mis-attributed,
	// or attributed to a series of different CLs as new data arrives.
	SettlingTime DurationAsString `json:"settling_time,omitempty"`
}

// FrontendFlags are the command-line flags for the web UI.
type FrontendFlags struct {
	ConfigFilename                 string
	ConnectionString               string
	CommitRangeURL                 string
	DefaultSparse                  bool
	DoClustering                   bool
	NoEmail                        bool
	EventDrivenRegressionDetection bool
	Interesting                    float64
	KeyOrder                       string
	Local                          bool
	NumContinuous                  int
	NumContinuousParallel          int
	NumShift                       int
	NumParamSetsForQueries         int
	Port                           string
	PromPort                       string
	ResourcesDir                   string
	InternalPort                   string
	Radius                         int
	StepUpOnly                     bool
	DisplayGroupBy                 bool
	HideListOfCommitsOnExplore     bool
	FetchChromePerfAnomalies       bool
	FeedbackURL                    string
}

// AsCliFlags returns a slice of cli.Flag.
//
// If clustering is true then this set of flags is for Clustering, as opposed to Frontend.
func (flags *FrontendFlags) AsCliFlags(clustering bool) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ConfigFilename,
			Name:        "config_filename",
			Value:       "./configs/nano.json",
			Usage:       "The name of the config file to use.",
		},
		&cli.StringFlag{
			Destination: &flags.ConnectionString,
			Name:        "connection_string",
			Value:       "",
			Usage:       " Override Usage: the connection_string in the config file.",
		},
		&cli.StringFlag{
			Destination: &flags.CommitRangeURL,
			Name:        "commit_range_url",
			Value:       "",
			Usage:       "A URI Usage: Template to be used for expanding details on a range of commits, from {begin} to {end} git hash. See cluster-summary2-sk.",
		},
		&cli.BoolFlag{
			Destination: &flags.DefaultSparse,
			Name:        "default_sparse",
			Value:       false,
			Usage:       "The default value for 'Sparse' in Alerts.",
		},
		&cli.BoolFlag{
			Destination: &flags.DoClustering,
			Name:        "do_clustering",
			Value:       clustering,
			Usage:       "If true then run continuous clustering over all the alerts.",
		},
		&cli.BoolFlag{
			Destination: &flags.NoEmail,
			Name:        "noemail",
			Value:       false,
			Usage:       "Do not send emails.",
		},
		&cli.BoolFlag{
			Destination: &flags.EventDrivenRegressionDetection,
			Name:        "event_driven_regression_detection",
			Value:       false,
			Usage:       "If true then regression detection is done based on PubSub events.",
		},
		&cli.Float64Flag{
			Destination: &flags.Interesting,
			Name:        "interesting",
			Value:       50.0,
			Usage:       "The threshold value beyond which StepFit.Regression values become interesting, i.e. they may indicate real regressions or improvements.",
		},
		&cli.StringFlag{
			Destination: &flags.KeyOrder,
			Name:        "key_order",
			Value:       "build_flavor,name,sub_result,source_type",
			Usage:       "The order that keys should be presented in for searching. All keys that don't appear here will appear after.",
		},
		&cli.BoolFlag{
			Destination: &flags.Local,
			Name:        "local",
			Value:       false,
			Usage:       "Running locally if true. As opposed to in production.",
		},
		&cli.IntFlag{
			Destination: &flags.NumContinuous,
			Name:        "num_continuous",
			Value:       50,
			Usage:       "The number of commits to do continuous clustering over looking for regressions.",
		},
		&cli.IntFlag{
			Destination: &flags.NumContinuousParallel,
			Name:        "num_continuous_parallel",
			Value:       3,
			Usage:       "The number of parallel copies of continuous clustering to run.",
		},
		&cli.IntFlag{
			Destination: &flags.NumShift,
			Name:        "num_shift",
			Value:       10,
			Usage:       "The number of commits the shift navigation buttons should jump.",
		},
		&cli.IntFlag{
			Destination: &flags.NumParamSetsForQueries,
			Name:        "num_paramsets_for_queries",
			Value:       2,
			Usage:       "The number of paramsets we gather to populate the query dialog.",
		},
		&cli.StringFlag{
			Destination: &flags.Port,
			Name:        "port",
			Value:       ":8000",
			Usage:       "HTTP service address (e.g., ':8000')",
		},
		&cli.StringFlag{
			Destination: &flags.PromPort,
			Name:        "prom_port",
			Value:       ":20000",
			Usage:       "Metrics service address (e.g., ':10110')",
		},
		&cli.StringFlag{
			Destination: &flags.InternalPort,
			Name:        "internal_port",
			Value:       ":9000",
			Usage:       "HTTP service address for internal clients, e.g. probers. No authentication on this port.",
		},
		&cli.StringFlag{
			Destination: &flags.ResourcesDir,
			Name:        "resources_dir",
			Value:       "",
			Usage:       "The directory to find templates, JS, and CSS files. If blank then ../../dist relative to the current directory will be used.",
		},
		&cli.IntFlag{
			Destination: &flags.Radius,
			Name:        "radius",
			Value:       7,
			Usage:       "The number of commits to include on either side of a commit when clustering.",
		},
		&cli.BoolFlag{
			Destination: &flags.StepUpOnly,
			Name:        "step_up_only",
			Value:       false,
			Usage:       "Only regressions that look like a step up will be reported.",
		},
		&cli.BoolFlag{
			Destination: &flags.DisplayGroupBy,
			Name:        "display_group_by",
			Value:       false,
			Usage:       "Show the Group By section of Alert configuration.",
		},
		&cli.BoolFlag{
			Destination: &flags.HideListOfCommitsOnExplore,
			Name:        "hide_list_of_commits_on_explore",
			Value:       false,
			Usage:       "Hide the commit-detail-panel-sk element on the Explore details tab.",
		},
		&cli.BoolFlag{
			Destination: &flags.FetchChromePerfAnomalies,
			Name:        "fetch_chrome_perf_anomalies",
			Value:       false,
			Usage:       "Fetch anomalies and show the bisect button",
		},
		&cli.StringFlag{
			Destination: &flags.FeedbackURL,
			Name:        "feedback_url",
			Value:       "",
			Usage:       "Feedback Url to display on the page",
		},
	}
}

// IngestFlags are the command-line flags for the ingestion process.
type IngestFlags struct {
	ConfigFilename       string
	ConnectionString     string
	PromPort             string
	Local                bool
	NumParallelIngesters int
}

// AsCliFlags returns a slice of cli.Flag.
func (flags *IngestFlags) AsCliFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.ConfigFilename,
			Name:        "config_filename",
			Value:       "",
			Usage:       "Instance config file. Must be supplied.",
		},
		&cli.StringFlag{
			Destination: &flags.ConnectionString,
			Name:        "connection_string",
			Value:       "",
			Usage:       " Override the connection_string in the config file.",
		},
		&cli.StringFlag{
			Destination: &flags.PromPort,
			Name:        "prom_port",
			Value:       ":20000",
			Usage:       "Metrics service address (e.g., ':20000')",
		},
		&cli.BoolFlag{
			Destination: &flags.Local,
			Name:        "local",
			Value:       false,
			Usage:       "True if running locally and not in production.",
		},
		&cli.IntFlag{
			Destination: &flags.NumParallelIngesters,
			Name:        "num_parallel_ingesters",
			Value:       10,
			Usage:       "The number of parallel Go routines to have ingesting.",
		},
	}
}

// InstanceConfig contains all the info needed by a Perf instance.
type InstanceConfig struct {
	// URL is the root URL at which this instance is available, for example: "https://example.com".
	URL string `json:"URL"`

	// Other domain names that are allowed to make cross-site requests to this instance.
	AllowedHosts []string `json:"allowed_hosts,omitempty"`

	// Contact is the best way to contact the team for this instance.
	Contact string `json:"contact"`

	// Customized invalid char regrex, regex must never accept ',' or '='.
	// because '=' and ',' are used to parse the Param key and value,
	// they can never be allowed.
	InvalidParamCharRegex string `json:"invalid_param_char_regex,omitempty"`

	// FetchChromePerfAnomalies if true means fetch anomalies from Chrome Perf
	FetchChromePerfAnomalies bool `json:"fetch_chrome_perf_anomalies,omitempty"`

	// Feedback URL to use for the "Provide Feedback" link
	FeedbackURL string `json:"feedback_url,omitempty"`

	// TraceSampleProportion is a float between 0.0 and 1.0 that determines
	// which percentage of traces get uploaded
	TraceSampleProportion float32 `json:"trace_sample_proportion,omitempty"`

	AuthConfig      AuthConfig      `json:"auth_config,omitempty"`
	DataStoreConfig DataStoreConfig `json:"data_store_config"`
	IngestionConfig IngestionConfig `json:"ingestion_config"`
	GitRepoConfig   GitRepoConfig   `json:"git_repo_config"`
	NotifyConfig    NotifyConfig    `json:"notify_config"`
	AnomalyConfig   AnomalyConfig   `json:"anomaly_config,omitempty"`
}

// Validate the config.
func (i InstanceConfig) Validate() error {
	if i.NotifyConfig.Notifications == notifytypes.MarkdownIssueTracker {
		if i.NotifyConfig.IssueTrackerAPIKeySecretProject == "" {
			return skerr.Fmt("issue_tracker_api_key_secret_project must be supplied when `notifications` is set to %q", i.NotifyConfig.Notifications)
		}
		if i.NotifyConfig.IssueTrackerAPIKeySecretName == "" {
			return skerr.Fmt("issue_tracker_api_key_secret_name must be supplied when `notifications` is set to %q", i.NotifyConfig.Notifications)
		}
	}

	if i.InvalidParamCharRegex != "" {
		re, err := regexp.Compile(i.InvalidParamCharRegex)
		if err != nil {
			return skerr.Wrapf(err, "compiling invalid_param_char_regex: %q", i.InvalidParamCharRegex)
		}
		if !re.MatchString(",") {
			return skerr.Fmt("invalid_param_char_regex must match ',' (comma).")
		}
		if !re.MatchString("=") {
			return skerr.Fmt("invalid_param_char_regex must match '=' (equals).")
		}
	}

	return nil
}

// InstanceConfigFromFile returns the deserialized JSON of an InstanceConfig
// found in filename.
//
// If there was an error loading the file a list of schema violations may be
// returned also.
func InstanceConfigFromFile(filename string) (*InstanceConfig, []string, error) {
	ctx := context.Background()
	var instanceConfig InstanceConfig
	var schemaViolations []string = nil

	// Validate config here.
	err := util.WithReadFile(filename, func(r io.Reader) error {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return skerr.Wrapf(err, "failed to read bytes")
		}
		schemaViolations, err = jsonschema.Validate(ctx, b, schema)
		if err != nil {
			return skerr.Wrapf(err, "file does not conform to schema")
		}
		return json.Unmarshal(b, &instanceConfig)
	})
	if err != nil {
		return nil, schemaViolations, skerr.Wrapf(err, "Filename: %s", filename)
	}
	err = instanceConfig.Validate()
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "Filename: %s", filename)
	}
	return &instanceConfig, nil, nil
}

// Config is the currently running config.
var Config *InstanceConfig

// Init loads the selected config by name.
func Init(filename string) error {
	cfg, schemaViolations, err := InstanceConfigFromFile(filename)
	if err != nil {
		for _, v := range schemaViolations {
			sklog.Error(v)
		}
		return skerr.Wrap(err)
	}
	Config = cfg
	return nil
}

func IsDeadLetterCollectionEnabled(instanceConfig *InstanceConfig) bool {
	return instanceConfig.IngestionConfig.SourceConfig.DeadLetterTopic != ""
}
