package alerts

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"go.skia.org/infra/go/paramtools"
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

func (c *Config) GroupedBy() []string {
	ret := []string{}
	for _, s := range strings.Split(c.GroupBy, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ret = append(ret, s)
	}
	return ret
}

type KeyValue struct {
	Key   string
	Value string
}

type Combination []KeyValue

func equal(sliceA, sliceB []int) bool {
	for i, a := range sliceA {
		if a != sliceB[i] {
			return false
		}
	}
	return true
}

func inc(a, limits []int) []int {
	ret := make([]int, len(a))
	_ = copy(ret, a)
	for i := len(a) - 1; i >= 0; i-- {
		ret[i] = ret[i] + 1
		if ret[i] <= limits[i] {
			break
		}
		ret[i] = 0
	}
	return ret
}

func toCombination(offsets []int, keys []string, ps paramtools.ParamSet) (Combination, error) {
	ret := Combination{}
	for i, offset := range offsets {
		key := keys[i]
		values, ok := ps[key]
		if !ok {
			return nil, fmt.Errorf("Key %q not found in ParamSet %#v", key, ps)
		}
		ret = append(ret, KeyValue{
			Key:   key,
			Value: values[offset],
		})
	}
	return ret, nil
}

func (c *Config) GroupCombinations(ps paramtools.ParamSet) ([]Combination, error) {
	limits := []int{}
	keys := c.GroupedBy()
	for _, key := range keys {
		limits = append(limits, len(ps[key])-1)
	}
	ret := []Combination{}
	zeroes := make([]int, len(limits))
	cfg := make([]int, len(limits))
	for {
		comb, err := toCombination(cfg, keys, ps)
		if err != nil {
			return nil, fmt.Errorf("Failed to build combination: %s", err)
		}
		ret = append(ret, comb)
		cfg = inc(cfg, limits)
		if equal(cfg, zeroes) {
			break
		}
	}
	return ret, nil
}

func (c *Config) Validate() error {
	parsed, err := url.ParseQuery(c.Query)
	if err != nil {
		return fmt.Errorf("Invalid Config: Invalid Query: %s", err)
	}
	if c.GroupBy != "" {
		for _, groupParam := range c.GroupedBy() {
			if _, ok := parsed[groupParam]; ok {
				return fmt.Errorf("Invalid Config: GroupBy must not appear in Query: %q %q ", c.GroupBy, c.Query)
			}
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
