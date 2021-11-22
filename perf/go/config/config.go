package config

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	cli "github.com/urfave/cli/v2"
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

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

	// LoginURL is the URL to redirect users to when they need to log in.
	LoginURL string `json:"login_url"`

	// LogoutURL is the URL to redirect users to when they need to log out.
	LogoutURL string `json:"logout_url"`
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

// GitRepoConfig is the config for the git repo.
type GitRepoConfig struct {
	// GitAuthType is the type of authentication the repo requires. Defaults to
	// GitAuthNone.
	GitAuthType GitAuthType `json:"git_auth_type,omitempty"`

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
}

// FrontendFlags are the command-line flags for the web UI.
type FrontendFlags struct {
	AuthBypassList                 string
	ConfigFilename                 string
	ConnectionString               string
	CommitRangeURL                 string
	DefaultSparse                  bool
	DoClustering                   bool
	NoEmail                        bool
	EmailClientSecretFile          string
	EmailTokenCacheFile            string
	EventDrivenRegressionDetection bool
	Interesting                    float64
	InternalOnly                   bool
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
	ProxyLogin                     bool
}

// AsCliFlags returns a slice of cli.Flag.
//
// If clustering is true then this set of flags is for Clustering, as opposed to Frontend.
func (flags *FrontendFlags) AsCliFlags(clustering bool) []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Destination: &flags.AuthBypassList,
			Name:        "auth_bypass_list",
			Value:       "",
			Usage:       "Space separated list of email addresses allowed access. Usually just service account emails. Bypasses the domain checks.",
		},
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
		&cli.StringFlag{
			Destination: &flags.EmailClientSecretFile,
			Name:        "email_client_secret_file",
			Value:       "client_secret.json",
			Usage:       "OAuth client secret JSON file for sending email.",
		},
		&cli.StringFlag{
			Destination: &flags.EmailTokenCacheFile,
			Name:        "email_token_cache_file",
			Value:       "client_token.json",
			Usage:       "OAuth token cache file for sending email.",
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
		&cli.BoolFlag{
			Destination: &flags.InternalOnly,
			Name:        "internal_only",
			Value:       false,
			Usage:       "Require the user to be logged in to see any page.",
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
			Destination: &flags.ProxyLogin,
			Name:        "proxy-login",
			Value:       false,
			Usage:       "Use //go/alogin/proxyauth, instead of the default of //go/alogin/sklogin, for verifying logged in users.",
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

	// Contact is the best way to contact the team for this instance.
	Contact string `json:"contact"`

	AuthConfig      AuthConfig      `json:"auth_config,omitempty"`
	DataStoreConfig DataStoreConfig `json:"data_store_config"`
	IngestionConfig IngestionConfig `json:"ingestion_config"`
	GitRepoConfig   GitRepoConfig   `json:"git_repo_config"`
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
