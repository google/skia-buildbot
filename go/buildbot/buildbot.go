package buildbot

/*
	Tools for loading data from Buildbot's JSON interface.
*/

const (
	BUILDBOT_URL  = "http://build.chromium.org/p/"
	LOAD_ATTEMPTS = 3
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

// GetProperty returns the key/value pair for a build property, if it exists,
// and nil otherwise.
func (b *Build) GetProperty(property string) interface{} {
	for _, propVal := range b.Properties {
		if propVal[0].(string) == property {
			return propVal
		}
	}
	return nil
}

// GotRevision returns the revision to which a build was synced, or the empty
// string if none.
func (b *Build) gotRevision() string {
	gotRevision := b.GetProperty("got_revision")
	if gotRevision == nil {
		return ""
	}
	if gotRevision.([]interface{})[1] == nil {
		return ""
	}
	return gotRevision.([]interface{})[1].(string)
}

// Branch returns the branch whose commit(s) triggered this build.
func (b *Build) branch() string {
	branch := b.GetProperty("branch")
	if branch == nil {
		return ""
	}
	if branch.([]interface{})[1] == nil {
		return ""
	}
	return branch.([]interface{})[1].(string)
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
		FailedSteps: steps,
		Finished:    b.IsFinished(),
		Id:          b.Id,
		Master:      b.Master,
		Number:      b.Number,
		Results:     b.Results,
	}
}

// BuildSummary is a struct which contains the minimal amount of information
// that we care to see on the dashboard.
type BuildSummary struct {
	Builder     string   `json:"builder"`
	FailedSteps []string `json:"failedSteps"`
	Finished    bool     `json:"finished"`
	Id          int      `json:"id"`
	Master      string   `json:"master"`
	Number      int      `json:"number"`
	Results     int      `json:"results"`
}
