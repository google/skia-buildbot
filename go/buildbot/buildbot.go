package buildbot

import (
	"fmt"
	"regexp"
	"strings"
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

// BuildID contains the minimum amount of information to identify a Build.
type BuildID struct {
	Builder string `db:"builder"`
	Master  string `db:"master"`
	Number  int    `db:"number"`
}

// BuildStep contains information about a build step.
type BuildStep struct {
	Id         int    `db:"id"`
	BuildID    int    `db:"buildId"`
	Name       string `db:"name"`
	Times      []float64
	Number     int           `json:"step_number" db:"number"`
	Results    int           `db:"results"`
	ResultsRaw []interface{} `json:"results"`
	Started    float64       `db:"started"`
	Finished   float64       `db:"finished"`
}

// Build.Results code descriptions, see http://docs.buildbot.net/current/developer/results.html.
const (
	BUILDBOT_SUCCESS = 0
	BUILDBOT_WARNING = 1
	BUILDBOT_FAILURE = 2
)

// Parse s as one of BUILDBOT_SUCCESS, BUILDBOT_WARNING, or BUILDBOT_FAILURE.
func ParseResultsString(s string) (int, error) {
	switch strings.ToLower(s) {
	case "success":
		return BUILDBOT_SUCCESS, nil
	case "warning":
		return BUILDBOT_WARNING, nil
	case "failure":
		return BUILDBOT_FAILURE, nil
	default:
		return 0, fmt.Errorf("Invalid buildbot Results code: %s", s)
	}
}

// Build contains information about a single build.
type Build struct {
	Id            int    `db:"id"`
	Builder       string `db:"builder"`
	Master        string `db:"master"`
	Number        int    `db:"number"`
	BuildSlave    string `db:"buildslave"`
	Branch        string `db:"branch"`
	Commits       []string
	GotRevision   string          `db:"gotRevision"`
	Properties    [][]interface{} `db:"_"`
	PropertiesStr string          `db:"properties"`
	Results       int             `db:"results"`
	Steps         []*BuildStep
	Times         []float64
	Started       float64 `db:"started"`
	Finished      float64 `db:"finished"`
	Comments      []*BuildComment
	Repository    string `db:"repository"`
}

// BuildSlave contains information about a buildslave.
type BuildSlave struct {
	Admin     string
	Builders  map[string][]int
	Connected bool
	Host      string
	Name      string
	Version   string
}

// BuildComment contains a comment about a build.
type BuildComment struct {
	Id        int     `db:"id"        json:"id"`
	BuildId   int     `db:"buildId"   json:"buildId"`
	User      string  `db:"user"      json:"user"`
	Timestamp float64 `db:"timestamp" json:"time"`
	Message   string  `db:"message"   json:"message"`
}

// BuilderComment contains a comment about a builder.
type BuilderComment struct {
	Id            int     `db:"id"            json:"id"`
	Builder       string  `db:"builder"       json:"builder"`
	User          string  `db:"user"          json:"user"`
	Timestamp     float64 `db:"timestamp"     json:"time"`
	Flaky         bool    `db:"flaky"         json:"flaky"`
	IgnoreFailure bool    `db:"ignoreFailure" json:"ignoreFailure"`
	Message       string  `db:"message"       json:"message"`
}

// CommitComment contains a comment about a commit.
type CommitComment struct {
	Id        int     `db:"id"        json:"id"`
	Commit    string  `db:"commit"    json:"commit"`
	User      string  `db:"user"      json:"user"`
	Timestamp float64 `db:"timestamp" json:"time"`
	Message   string  `db:"message"   json:"message"`
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
	strVal, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("Not a string property %s", property)
	}
	return strVal, nil
}

// GotRevision returns the revision to which a build was synced, or the empty
// string if none.
func (b *Build) gotRevision() string {
	if gotRevision, err := b.GetStringProperty("got_revision"); err == nil {
		return gotRevision
	} else {
		return ""
	}
}

// Branch returns the branch whose commit(s) triggered this build.
func (b *Build) branch() string {
	if branch, err := b.GetStringProperty("branch"); err == nil {
		return branch
	} else {
		return ""
	}
}

// Repository returns the repository whose commit(s) triggered this build.
func (b *Build) repository() string {
	if repo, err := b.GetStringProperty("repository"); err == nil {
		return repo
	} else {
		return ""
	}
}

// Finished indicates whether the build has finished.
func (b *Build) IsFinished() bool {
	return b.Finished != 0.0
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
		Id:          b.Id,
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
	Id          int             `json:"id"`
	Master      string          `json:"master"`
	Number      int             `json:"number"`
	Properties  [][]interface{} `json:"properties"`
	Results     int             `json:"results"`
	Comments    []*BuildComment `json:"comments"`
	Commits     []string        `json:"commits"`
}
