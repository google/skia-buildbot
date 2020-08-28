package config

import (
	"encoding/json"
	"io"

	"github.com/spf13/pflag"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

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
	CacheConfig CacheConfig
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
	RejectIfNameMatches string `json:"reject_if_name_matches"`

	// AcceptIfNameMatches is a regex. If it matches the file.Name the file will
	// be processed. Leave the empty string to accept all files.
	AcceptIfNameMatches string `json:"accept_if_name_matches"`
}

// IngestionConfig is the configuration for how source files are ingested into
// being traces in a TraceStore.
type IngestionConfig struct {
	// SourceConfig is the config for where files to ingest come from.
	SourceConfig SourceConfig `json:"source_config"`

	// Branches, if populated then restrict to ingesting just these branches.
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
	// GitAuthType is the type of authentication the repo requires.
	GitAuthType GitAuthType `json:"git_auth_type"`

	// URL that the Git repo is fetched from.
	URL string `json:"url"`

	// Dir is the directory into which the repo should be checked out.
	Dir string `json:"dir"`

	// FileChangeMarker is a path in the git repo to watch for changes. If the
	// file indicated changes in a commit then a marker will be displayed on the
	// graph at that commit.
	FileChangeMarker string `json:"file_change_marker"`

	// DebouceCommitURL signals if a link to a Git commit needs to be specially
	// dereferenced. That is, some repos are synthetic and just contain a single
	// file that changes, with a commit message that is a URL that points to the
	// true source of information. If this value is true then links to commits
	// need to be debounced and use the commit message instead.
	DebouceCommitURL bool `json:"debounce_commit_url"`

	// CommitURL is a Go format string that joins the GitRepoConfig URL with a
	// commit hash to produce the URL of a web page that shows that exact
	// commit. For example "%s/commit/%s" would be a good value for GitHub
	// repos, while "%s/+show/%s" is a good value for Gerrit repos. Defaults
	// to "%s/+show/%s" if no value is supplied.
	CommitURL string `json:"commit_url"`
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
	Port                           string
	PromPort                       string
	InternalPort                   string
	Radius                         int
	StepUpOnly                     bool
}

// Register the flags in the given FlagSet.
func (flags *FrontendFlags) Register(fs *pflag.FlagSet) {
	fs.StringVar(&flags.AuthBypassList, "auth_bypass_list", "", "Space separated list of email addresses allowed access. Usually just service account emails. Bypasses the domain checks.")
	fs.StringVar(&flags.ConfigFilename, "config_filename", "./configs/nano.json", "The name of the config file to use.")
	fs.StringVar(&flags.ConnectionString, "connection_string", "", " Override the connection_string in the config file.")
	fs.StringVar(&flags.CommitRangeURL, "commit_range_url", "", "A URI Template to be used for expanding details on a range of commits, from {begin} to {end} git hash. See cluster-summary2-sk.")
	fs.BoolVar(&flags.DefaultSparse, "default_sparse", false, "The default value for 'Sparse' in Alerts.")
	fs.BoolVar(&flags.DoClustering, "do_clustering", true, "If true then run continuous clustering over all the alerts.")
	fs.BoolVar(&flags.NoEmail, "noemail", false, "Do not send emails.")
	fs.StringVar(&flags.EmailClientSecretFile, "email_client_secret_file", "client_secret.json", "OAuth client secret JSON file for sending email.")
	fs.StringVar(&flags.EmailTokenCacheFile, "email_token_cache_file", "client_token.json", "OAuth token cache file for sending email.")
	fs.BoolVar(&flags.EventDrivenRegressionDetection, "event_driven_regression_detection", false, "If true then regression detection is done based on PubSub events.")
	fs.Float64Var(&flags.Interesting, "interesting", 50.0, "The threshold value beyond which StepFit.Regression values become interesting, i.e. they may indicate real regressions or improvements.")
	fs.BoolVar(&flags.InternalOnly, "internal_only", false, "Require the user to be logged in to see any page.")
	fs.StringVar(&flags.KeyOrder, "key_order", "build_flavor,name,sub_result,source_type", "The order that keys should be presented in for searching. All keys that don't appear here will appear after.")
	fs.BoolVar(&flags.Local, "local", false, "Running locally if true. As opposed to in production.")
	fs.IntVar(&flags.NumContinuous, "num_continuous", 50, "The number of commits to do continuous clustering over looking for regressions.")
	fs.IntVar(&flags.NumContinuousParallel, "num_continuous_parallel", 3, "The number of parallel copies of continuous clustering to run.")
	fs.IntVar(&flags.NumShift, "num_shift", 10, "The number of commits the shift navigation buttons should jump.")
	fs.StringVar(&flags.Port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&flags.PromPort, "prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.StringVar(&flags.InternalPort, "internal_port", ":9000", "HTTP service address for internal clients, e.g. probers. No authentication on this port.")
	fs.IntVar(&flags.Radius, "radius", 7, "The number of commits to include on either side of a commit when clustering.")
	fs.BoolVar(&flags.StepUpOnly, "step_up_only", false, "Only regressions that look like a step up will be reported.")
}

// IngestFlags are the command-line flags for the ingestion process.
type IngestFlags struct {
	ConfigFilename       string
	ConnectionString     string
	PromPort             string
	Local                bool
	NumParallelIngesters int
}

// Register the flags in the given FlagSet.
func (flags *IngestFlags) Register(fs *pflag.FlagSet) {
	fs.StringVar(&flags.ConfigFilename, "config_filename", "", "Instance config file. Must be supplied.")
	fs.StringVar(&flags.ConnectionString, "connection_string", "", " Override the connection_string in the config file.")
	fs.StringVar(&flags.PromPort, "prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	fs.BoolVar(&flags.Local, "local", false, "True if running locally and not in production.")
	fs.IntVar(&flags.NumParallelIngesters, "num_parallel_ingesters", 10, "The number of parallel Go routines to have ingesting.")
}

// InstanceConfig contains all the info needed by a Perf instance.
type InstanceConfig struct {
	// URL is the root URL at which this instance is available, for example: "https://example.com".
	URL string `json:"URL"`

	DataStoreConfig DataStoreConfig `json:"data_store_config"`
	IngestionConfig IngestionConfig `json:"ingestion_config"`
	GitRepoConfig   GitRepoConfig   `json:"git_repo_config"`
}

// InstanceConfigFromFile returns the deserialized JSON of an InstanceConfig found in filename.
func InstanceConfigFromFile(filename string) (*InstanceConfig, error) {
	var instanceConfig InstanceConfig

	err := util.WithReadFile(filename, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&instanceConfig)
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &instanceConfig, nil
}

// Config is the currently running config.
var Config *InstanceConfig

// Init loads the selected config by name and then populated the Flags from the
// given flags.
func Init(filename string) error {
	cfg, err := InstanceConfigFromFile(filename)
	if err != nil {
		return skerr.Wrap(err)
	}
	Config = cfg
	return nil
}
