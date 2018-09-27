package alerts

import (
	"fmt"
	"net/url"
	"strconv"

	"go.skia.org/infra/perf/go/clustering2"
)

const (
	INVALID_ID = -1
)

var (
	// The default value for Config.Sparse.
	DefaultSparse = false
)

// Config represents the configuration for one alert.
type Config struct {
	ID             int64                   `json:"id"               datastore:",noindex"`
	DisplayName    string                  `json:"display_name"     datastore:",noindex"`
	Query          string                  `json:"query"            datastore:",noindex"` // The query to perform on the trace store to select the traces to alert on.
	Alert          string                  `json:"alert"            datastore:",noindex"` // Email address or id of a chat room to send alerts to.
	Interesting    float32                 `json:"interesting"      datastore:",noindex"` // The regression interestingness threshold.
	BugURITemplate string                  `json:"bug_uri_template" datastore:",noindex"` // URI Template used for reporting bugs. Format TBD.
	Algo           clustering2.ClusterAlgo `json:"algo"             datastore:",noindex"` // Which clustering algorithm to use.
	State          ConfigState             `json:"state"`                                 // The state of the config.
	Owner          string                  `json:"owner"            datastore:",noindex"` // Email address of the person that owns this alert.
	StepUpOnly     bool                    `json:"step_up_only"     datastore:",noindex"` // If true then only steps up will trigger an alert. [Deprecated, use Direction.]
	Direction      Direction               `json:"direction"        datastore:",noindex"` // Which direction will trigger an alert.
	Radius         int                     `json:"radius"           datastore:",noindex"` // How many commits to each side of a commit to consider when looking for a step. 0 means use the server default.
	K              int                     `json:"k"                datastore:",noindex"` // The K in k-means clustering. 0 means use an algorithmically chosen value based on the data.
	GroupBy        string                  `json:"group_by"         datastore:",noindex"` // A key in the paramset that all Clustering should be broken up across. Key must not appear in Query.
	Sparse         bool                    `json:"sparse"           datastore:",noindex"` // Data is sparse, so only include commits that have data.
	MinimumNum     int                     `json:"minimum_num"      datastore:",noindex"` // How many traces need to be found interesting before an alert is fired.
	Category       string                  `json:"category"         datastore:",noindex"` // Which category this alert falls into.
}

func (c *Config) IdAsString() string {
	return fmt.Sprintf("%d", c.ID)
}

func (c *Config) StringToId(s string) {
	if i, err := strconv.ParseInt(s, 10, 64); err != nil {
		c.ID = -1
	} else {
		c.ID = i
	}
}

func (c *Config) Validate() error {
	parsed, err := url.ParseQuery(c.Query)
	if err != nil {
		return fmt.Errorf("Invalid Config: Invalid Query: %s", err)
	}
	if c.GroupBy != "" {
		if _, ok := parsed[c.GroupBy]; ok {
			return fmt.Errorf("Invalid Config: GroupBy must not appear in Query: %q %q ", c.GroupBy, c.Query)
		}
	}
	if c.StepUpOnly {
		c.StepUpOnly = false
		c.Direction = UP
	}
	return nil
}

// NewConfig creates a new Config properly initialized.
func NewConfig() *Config {
	return &Config{
		ID:     INVALID_ID,
		Algo:   clustering2.KMEANS_ALGO,
		State:  ACTIVE,
		Sparse: DefaultSparse,
	}
}
