package buildbot

/*
	Tools for loading data from Buildbot's JSON interface.
*/

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
)

const (
	buildbotUrl  = "http://build.chromium.org/p/"
	loadAttempts = 3
)

var (
	MasterNames = []string{"client.skia", "client.skia.android", "client.skia.compile", "client.skia.fyi"}
)

// BuildStep contains information about a build step.
type BuildStep struct {
	IsFinished bool
	IsStarted  bool
	Name       string
	Logs       [][]string
	Times      []float64
	Text       []string
	Number     int `json:"step_number"`
	Results    []interface{}
}

// Build contains information about a single build.
type Build struct {
	BuilderName string
	Blame       []string
	Branch      string
	GotRevision string
	MasterName  string
	Number      int
	Properties  [][]interface{}
	Results     int
	Steps       []*BuildStep
	Times       []float64
}

// GetProperty returns the key/value pair for a build property, if it exists,
// and nil otherwise.
func (b Build) GetProperty(property string) interface{} {
	for _, propVal := range b.Properties {
		if propVal[0].(string) == property {
			return propVal
		}
	}
	return nil
}

// GotRevision returns the revision to which a build was synced, or the empty
// string if none.
func (b Build) gotRevision() string {
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
func (b Build) branch() string {
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
func (b Build) Finished() bool {
	return b.Times[1] != 0.0
}

// Builder contains information about a single builder.
type Builder struct {
	CachedBuilds []int
	Builds       []*Build
	Name         string
	Master       *Master
}

// get loads data from a buildbot JSON endpoint.
func get(url string, rv interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Failed to GET %s: %v", url, err)
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(rv); err != nil {
		return fmt.Errorf("Failed to decode JSON: %v", err)
	}
	return nil
}

// GetBuild retrieves the given build as specified by the master, builder, and
// build number.
func GetBuild(master, builder string, buildNumber int) (*Build, error) {
	var build Build
	url := fmt.Sprintf("%s%s/json/builders/%s/builds/%d", buildbotUrl, master, builder, buildNumber)
	err := get(url, &build)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve build #%v for %v: %v", buildNumber, builder, err)
	}
	build.Branch = build.branch()
	build.GotRevision = build.gotRevision()
	build.MasterName = master
	return &build, nil
}

// getBuilds returns a slice of new builds for the given builder.
func getBuilds(m *Master, b *Builder) ([]*Build, error) {
	lastBuild := -1
	if len(b.Builds) > 0 {
		lastBuild = b.Builds[len(b.Builds)-1].Number
	}
	var buildsToLoad []int
	for i, buildNum := range b.CachedBuilds {
		if buildNum > lastBuild {
			buildsToLoad = b.CachedBuilds[i:]
			break
		}
	}

	builds := make([]*Build, len(buildsToLoad), len(buildsToLoad))
	var fail error
	var wg sync.WaitGroup
	for i, n := range buildsToLoad {
		wg.Add(1)
		go func(index, buildNumber int) {
			defer wg.Done()
			var err error
			for attempt := 0; attempt < loadAttempts; attempt++ {
				build, err := GetBuild(m.Name, b.Name, buildNumber)
				if err == nil {
					builds[index] = build
					return
				}
				time.Sleep(500 * time.Millisecond)
				glog.Infof("Retrying %v #%v (attempt #%v)", b.Name, buildNumber, attempt+2)
			}
			fail = fmt.Errorf("Failed to load %v #%v after %d attempts: %v", b.Name, buildNumber, loadAttempts, err)
		}(i, n)
	}
	wg.Wait()
	glog.Infof("Done with %s", b.Name)
	if fail != nil {
		return nil, fail
	}
	return builds, nil
}

// Master contains information about a single build master.
type Master struct {
	Name     string
	Builders map[string]*Builder
}

// Reload refreshes the data contained in the given Master object.
func (m *Master) Reload() error {
	b, err := GetBuilders(m.Name)
	if err != nil {
		return fmt.Errorf("Could not load master %s: %v", m.Name, err)
	}
	for name, builder := range b {
		builder.Name = name
		builder.Master = m
		if _, ok := m.Builders[name]; !ok {
			m.Builders[name] = builder
		}
	}

	var fail error
	var wg sync.WaitGroup
	for _, builder := range m.Builders {
		wg.Add(1)
		go func(b *Builder) {
			defer wg.Done()
			builds, err := getBuilds(m, b)
			if err != nil {
				fail = err
			} else {
				b.Builds = append(b.Builds, builds...)
			}
		}(builder)
	}
	wg.Wait()
	glog.Infof("Done with %s", m.Name)
	if fail != nil {
		return fmt.Errorf("Failed to load builds: %v", fail)
	}
	return nil
}

// GetBuilders returns data about the builders on the given build master, in
// the form of a map whose keys are the builder names and values are Builder
// objects.
func GetBuilders(masterName string) (map[string]*Builder, error) {
	builders := map[string]*Builder{}
	err := get(buildbotUrl+masterName+"/json/builders", &builders)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve builders for %v: %v", masterName, err)
	}
	return builders, nil
}

// BuildbotData contains information about a set of build masters.
type BuildbotData struct {
	Masters []*Master
}

// Reload refreshes the data contained in the given BuildbotData object.
func (d *BuildbotData) Reload() error {
	var fail error
	var wg sync.WaitGroup
	for _, master := range d.Masters {
		wg.Add(1)
		go func(m *Master) {
			defer wg.Done()
			err := m.Reload()
			if err != nil {
				fail = err
			}
		}(master)
	}
	wg.Wait()
	return fail
}

// LoadBuildbotData returns an up-to-date BuildbotData object containing
// information about builds for all builders on all build masters.
func LoadBuildbotData() (*BuildbotData, error) {
	masters := make([]*Master, len(MasterNames), len(MasterNames))
	for i, name := range MasterNames {
		masters[i] = &Master{name, map[string]*Builder{}}
	}
	d := BuildbotData{Masters: masters}
	return &d, d.Reload()
}
