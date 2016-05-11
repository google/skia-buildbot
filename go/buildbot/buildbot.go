package buildbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/util"
)

/*
	Tools for loading data from Buildbot's JSON interface.
*/

const (
	BUILDBOT_URL  = "http://build.chromium.org/p/"
	LOAD_ATTEMPTS = 3
)

var (
	TRYBOT_REGEXP = regexp.MustCompile(".*-Trybot$")
)

// BuildStep contains information about a build step.
type BuildStep struct {
	Name     string
	Number   int
	Results  int
	Started  time.Time
	Finished time.Time
}

// IsStarted returns true iff the BuildStep has started.
func (bs *BuildStep) IsStarted() bool {
	return !util.TimeIsZero(bs.Started)
}

// IsFinished returns true iff the BuildStep has finished.
func (bs *BuildStep) IsFinished() bool {
	return !util.TimeIsZero(bs.Finished)
}

// Build.Results code descriptions, see http://docs.buildbot.net/current/developer/results.html.
const (
	BUILDBOT_SUCCESS   = 0
	BUILDBOT_WARNINGS  = 1
	BUILDBOT_FAILURE   = 2
	BUILDBOT_SKIPPED   = 3
	BUILDBOT_EXCEPTION = 4
	// The doc above says that both EXCEPTION and RETRY are value 4.
	BUILDBOT_RETRY = 4
)

// Parse s as one of the above values for Build.Results.
func ParseResultsString(s string) (int, error) {
	switch strings.ToLower(s) {
	case "success":
		return BUILDBOT_SUCCESS, nil
	case "warnings", "warning":
		return BUILDBOT_WARNINGS, nil
	case "failure":
		return BUILDBOT_FAILURE, nil
	case "skipped":
		return BUILDBOT_SKIPPED, nil
	case "exception":
		return BUILDBOT_EXCEPTION, nil
	case "retry":
		return BUILDBOT_RETRY, nil
	default:
		return 0, fmt.Errorf("Invalid buildbot Results code: %s", s)
	}
}

// Build contains information about a single build.
type Build struct {
	Builder       string
	Master        string
	Number        int
	BuildSlave    string
	Branch        string
	Commits       []string
	GotRevision   string
	Properties    [][]interface{}
	PropertiesStr string
	Results       int
	Steps         []*BuildStep
	Started       time.Time
	Finished      time.Time
	Comments      []*BuildComment
	Repository    string
}

// Id constructs an ID for the given Build.
func (b *Build) Id() BuildID {
	return MakeBuildID(b.Master, b.Builder, b.Number)
}

// jsonBuildStep is a struct used for (de)serializing a BuildStep to JSON.
type jsonBuildStep struct {
	Name    string        `json:"name"`
	Times   []float64     `json:"times"`
	Number  int           `json:"step_number"`
	Results []interface{} `json:"results"`
}

// jsonBuild is a struct used for (de)serializing a Build to JSON.
type jsonBuild struct {
	Builder    string           `json:"builderName"`
	Number     int              `json:"number"`
	Properties [][]interface{}  `json:"properties"`
	Results    int              `json:"results"`
	Steps      []*jsonBuildStep `json:"steps"`
	Times      []float64        `json:"times"`
}

// MarshalJSON serializes the Build to JSON.
func (b *Build) MarshalJSON() ([]byte, error) {
	build := jsonBuild{
		Builder: b.Builder,
		Number:  b.Number,
		Results: b.Results,
		Times: []float64{
			util.TimeToUnixFloat(b.Started),
			util.TimeToUnixFloat(b.Finished),
		},
		Properties: b.Properties,
	}

	steps := make([]*jsonBuildStep, 0, len(b.Steps))
	for _, s := range b.Steps {
		steps = append(steps, &jsonBuildStep{
			Name:   s.Name,
			Number: s.Number,
			Results: []interface{}{
				s.Results,
				[]interface{}{},
			},
			Times: []float64{
				util.TimeToUnixFloat(s.Started),
				util.TimeToUnixFloat(s.Finished),
			},
		})
	}

	build.Steps = steps

	return json.Marshal(&build)
}

// UnmarshalJSON deserializes the Build from JSON.
func (b *Build) UnmarshalJSON(data []byte) error {
	var build jsonBuild
	if err := json.NewDecoder(bytes.NewBuffer(data)).Decode(&build); err != nil {
		return err
	}

	b.Builder = build.Builder
	b.Number = build.Number
	b.Properties = build.Properties
	b.Results = build.Results
	if len(build.Times) != 2 {
		return fmt.Errorf("times array must have length 2: %v", build.Times)
	}
	b.Started = util.UnixFloatToTime(build.Times[0])
	b.Finished = util.UnixFloatToTime(build.Times[1])

	// Parse the following from build properties.
	var err error
	b.Repository, err = b.GetStringProperty("repository")
	if err != nil {
		return err
	}
	b.GotRevision, err = b.GetStringProperty("got_revision")
	if err != nil {
		b.GotRevision = ""
	}
	b.Branch, err = b.GetStringProperty("branch")
	if err != nil {
		return err
	}
	b.BuildSlave, err = b.GetStringProperty("slavename")
	if err != nil {
		return err
	}
	b.Master, err = b.GetStringProperty("mastername")
	if err != nil {
		return err
	}

	b.Steps = make([]*BuildStep, 0, len(build.Steps))
	for _, s := range build.Steps {
		if len(s.Times) != 2 {
			return fmt.Errorf("times array must have length 2 (step): %v", s.Times)
		}

		results := 0
		if len(s.Results) > 0 {
			if s.Results[0] != nil {
				results = int(s.Results[0].(float64))
			}
		}

		b.Steps = append(b.Steps, &BuildStep{
			Name:     s.Name,
			Number:   s.Number,
			Results:  results,
			Started:  util.UnixFloatToTime(s.Times[0]).UTC(),
			Finished: util.UnixFloatToTime(s.Times[1]).UTC(),
		})
	}
	b.fixup()

	return nil
}

// fixup fixes a Build object before/after deserialization.
func (b *Build) fixup() {
	// gob considers empty slices and nil slices to be the same. Create
	// empty slices for any that might be nil.
	if reflect.ValueOf(b.Comments).IsNil() {
		b.Comments = []*BuildComment{}
	}
	if reflect.ValueOf(b.Commits).IsNil() {
		b.Commits = []string{}
	}
	if reflect.ValueOf(b.Steps).IsNil() {
		b.Steps = []*BuildStep{}
	}

	// Ensure that all times are in UTC.
	b.Started = b.Started.UTC()
	b.Finished = b.Finished.UTC()
	for _, s := range b.Steps {
		s.Started = s.Started.UTC()
		s.Finished = s.Finished.UTC()
	}
	for _, c := range b.Comments {
		c.Timestamp = c.Timestamp.UTC()
	}

	// Sort the commits alphabetically, for convenience.
	sort.Strings(b.Commits)
}

// Builder contains information about a builder.
type Builder struct {
	Name          string
	Master        string
	PendingBuilds int
	Slaves        []string
	State         string
}

// BuildSlave contains information about a buildslave.
type BuildSlave struct {
	Builders      map[string][]int
	Connected     bool
	Name          string
	Master        string
	RunningBuilds []interface{}
}

// BuildComment contains a comment about a build.
type BuildComment struct {
	Id        int64     `json:"id"`
	User      string    `json:"user"`
	Timestamp time.Time `json:"time"`
	Message   string    `json:"message"`
}

// BuilderComment contains a comment about a builder.
type BuilderComment struct {
	Id            int64     `json:"id"`
	Builder       string    `json:"builder"`
	User          string    `json:"user"`
	Timestamp     time.Time `json:"time"`
	Flaky         bool      `json:"flaky"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message"`
}

// CommitComment contains a comment about a commit.
type CommitComment struct {
	Id        int64     `json:"id"`
	Commit    string    `json:"commit"`
	User      string    `json:"user"`
	Timestamp time.Time `json:"time"`
	Message   string    `json:"message"`
}

// IsTrybot determines whether the given builder is a trybot.
func IsTrybot(b string) bool {
	return TRYBOT_REGEXP.MatchString(b)
}

// GetProperty returns the value for a build property, if it exists. Otherwise returns an error.
func (b *Build) GetProperty(property string) (interface{}, error) {
	for _, propVal := range b.Properties {
		if len(propVal) >= 2 {
			key, ok := propVal[0].(string)
			if ok && key == property {
				return propVal[1], nil
			}
		}
	}
	return nil, fmt.Errorf("No such property %s", property)
}

// GetStringProperty returns the value for a build property if it exists and it is a string. Otherwise returns an error.
func (b *Build) GetStringProperty(property string) (string, error) {
	val, err := b.GetProperty(property)
	if err != nil {
		return "", err
	}
	// It's okay for the property to be unset.
	if val == nil {
		return "", nil
	}

	strVal, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("Not a string property %s", property)
	}
	return strVal, nil
}

// getPropertyInterface returns an interface value for the given property.
func getPropertyInterface(propname string, value interface{}) []interface{} {
	return []interface{}{
		propname,
		value,
		"fake_source",
	}
}

// IsStarted indicates whether the build has started.
func (b *Build) IsStarted() bool {
	return !util.TimeIsZero(b.Started)
}

// IsFinished indicates whether the build has finished.
func (b *Build) IsFinished() bool {
	return !util.TimeIsZero(b.Finished)
}

// GetSummary returns a BuildSummary for the given Build.
func (b *Build) GetSummary() *BuildSummary {
	steps := make([]string, 0, len(b.Steps))
	for _, s := range b.Steps {
		if s.Results != 0 && s.Name != "steps" {
			steps = append(steps, s.Name)
		}
	}
	return &BuildSummary{
		Builder:     b.Builder,
		BuildSlave:  b.BuildSlave,
		FailedSteps: steps,
		Finished:    b.IsFinished(),
		Master:      b.Master,
		Number:      b.Number,
		Properties:  b.Properties,
		Results:     b.Results,
		Comments:    b.Comments,
		Commits:     b.Commits,
	}
}

// BuildSummary is a struct which contains the minimal amount of information
// that we care to see on the dashboard.
type BuildSummary struct {
	Builder     string          `json:"builder"`
	BuildSlave  string          `json:"buildslave"`
	FailedSteps []string        `json:"failedSteps"`
	Finished    bool            `json:"finished"`
	Master      string          `json:"master"`
	Number      int             `json:"number"`
	Properties  [][]interface{} `json:"properties"`
	Results     int             `json:"results"`
	Comments    []*BuildComment `json:"comments"`
	Commits     []string        `json:"commits"`
}
