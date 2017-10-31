package bbtrybot

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	// REMOVE REMOVE REMOVE

	_ "go.chromium.org/luci/buildbucket"
	bb_api "go.chromium.org/luci/common/api/buildbucket/buildbucket/v1"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
)

const (
	TIME_WINDOW         = 5 * 24 * time.Hour // Time we look back at the build bucket
	POLL_INTERVAL       = 5 * time.Minute    // Interval at which we poll buildbucket
	TEST_BUILDER_PREFIX = "Test-"            // Prefix that identifies Trybots that run tests/produce images.
)

//  struct {
// 	BuilderName string     `json:"builder_name"`
// 	Changes     []*Change  `json:"changes"`
// 	Properties  Properties `json:"properties"`
// 	Swarming    *swarming  `json:"swarming,omitempty"`
// }

type TrybotState struct {
	service     *bb_api.Service
	trybotStore TrybotStore
	gerritAPI   *gerrit.Gerrit
}

func NewTrybotState(client *http.Client, trybotStore TrybotStore) (*TrybotState, error) {
	service, err := bb_api.New(client)
	if err != nil {
		return nil, err
	}
	service.BasePath = "https://cr-buildbucket.appspot.com/api/buildbucket/v1/"
	return &TrybotState{
		service: service,
	}, nil
}

func (t *TrybotState) start() {
	buildsCh := make(chan *bb_api.ApiCommonBuildMessage)
	t.startBuildPoller(buildsCh, POLL_INTERVAL, TIME_WINDOW)

	go func() {
		for build := range buildsCh {
			params := &Parameters{}
			if err := json.Unmarshal([]byte(build.ParametersJson), params); err != nil {
				sklog.Errorf("Error unmarshalling params: %s", err)
				continue
			}

			// Check if this is a builder we can ignore.
			if t.ignoreBuild(build, params) {

				continue
			}

			if err := t.addToIssue(build, params); err != nil {
				sklog.Errorf("Error adding build infor to trybot store: %s", err)
				continue
			}
		}
	}()
}

func (t *TrybotState) addToIssue(build *bb_api.ApiCommonBuildMessage, params *Parameters) error {
	issueID := int64(params.Properties.GerritIssue)

	// Find the existing issue.
	issue, err := t.trybotStore.GetIssue(issueID, nil)
	if err != nil {
		return err
	}

	// If we didn't find the issue then create a new one.
	if issue == nil {
		issue, err = t.fetchGerritIssue(issueID)

		// Do soemthing if the issue doesn't exist.
		if err != nil {
			return err
		}
	}

	changed := issue.addBuild(build, params)
}

func (t *TrybotState) fetchGerritIssue(issueID int64) (*IssueDetails, error) {
	changeInfo, err := t.gerritAPI.GetIssueProperties(issueID)
	if err != nil {
		return nil, err
	}

	ret := &IssueDetails{
		Issue: &Issue{
			ID:        issueID,
			Subject:   changeInfo.Subject,
			Owner:     changeInfo.Owner.Email,
			Updated:   changeInfo.Updated,
			Patchsets: changeInfo.GetPatchsetIDs(),
		},
		PatchSetDetails:
	}

	// issue := &IssueDetails{
	// 	ID        string  `json:"id"`
	// 	Subject   string  `json:"subject"`
	// 	Owner     string  `json:"owner"`
	// 	Updated   int64   `json:"updated"`
	// 	URL       string  `json:"url"`
	// 	Patchsets []int64 `json:"patchsets"`
	// fmt.Printf("build: %d %s %d \n\n%s\n\n", oneBuild.Id, oneBuild.CreatedBy, oneBuild.CreatedTs, spew.Sdump(params))
	// fmt.Printf("build: \n\n %s \n\n\n", spew.Sdump(oneBuild))
	return nil, nil
}

func (t *TrybotState) startBuildPoller(buildsCh chan<- *bb_api.ApiCommonBuildMessage, interval, timeWindow time.Duration) {
	t.pollBuildBucket(buildsCh, timeWindow)
	go func() {
		for range time.Tick(interval) {
			t.pollBuildBucket(buildsCh, timeWindow)
		}
	}()
}

func (t *TrybotState) pollBuildBucket(buildsCh chan<- *bb_api.ApiCommonBuildMessage, timeWindow time.Duration) {
	// Search over a specific time window.
	searchCall := t.service.Search()

	timeWindowStart := time.Now().Add(timeWindow).UnixNano() / int64(time.Microsecond)
	searchCall.Bucket("skia.primary").CreationTsLow(timeWindowStart)

	if err := searchCall.Run(buildsCh, 0, nil); err != nil {
		sklog.Errorf("Error querying build bucket: %s", err)
	} else {
		sklog.Infof("Done successfully searching buildbucket.")
	}
}

func (t *TrybotState) ignoreBuild(build *bb_api.ApiCommonBuildMessage, params *Parameters) bool {
	return !strings.HasPrefix(params.BuilderName, TEST_BUILDER_PREFIX)
}
