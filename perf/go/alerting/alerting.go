package alerting

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"

	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/clustering"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/db"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

const (
	APIKEY_METADATA_KEY = "apikey"
	CLUSTER_SIZE        = 50
	CLUSTER_STDDEV      = 0.001
)

// CombineClusters combines freshly found clusters with existing clusters.
//
//  Algorithm:
//    Run clustering and pick out the "Interesting" clusters.
//    Compare all the Interesting clusters to all the existing relevant clusters,
//      where "relevant" clusters are ones whose Hash/timestamp of the step
//      exists in the current tile.
//    Start with an empty "list".
//    For each cluster:
//      For each relevant existing cluster:
//        Take the top 20 keys from the existing cluster and count how many appear
//        in the cluster.
//      If there are no matches then this is a new cluster, add it to the "list".
//      If there are matches, possibly to multiple existing clusters, find the
//      existing cluster with the most matches.
//        Take the cluster (old/new) with the most members, or the best fit if
//        they have the same number of matches.
//    Return all the updated clusters.
func CombineClusters(freshSummaries, oldSummaries []*types.ClusterSummary) []*types.ClusterSummary {
	ret := []*types.ClusterSummary{}

	stillFresh := []*types.ClusterSummary{}
	// If two cluster summaries have the same hash and same Regression direction
	// then they are the same, merge them together.
	for _, fresh := range freshSummaries {
		for _, old := range oldSummaries {
			if fresh.Hash == old.Hash && math.Signbit(fresh.StepFit.Regression) == math.Signbit(old.StepFit.Regression) {
				old.Merge(fresh)
				ret = append(ret, old)
				break
			}
		}
		stillFresh = append(stillFresh, fresh)
	}

	// Even if a summary has a different hash it might still be the same event if
	// there is an overlap in the traces each summary contains.
	for _, fresh := range stillFresh {
		var bestMatch *types.ClusterSummary = nil
		bestMatchHits := 0
		for _, old := range oldSummaries {
			hits := 0
			for _, key := range util.AtMost(old.Keys, 20) {
				if util.In(key, fresh.Keys) {
					hits += 1
				}
			}
			if hits > bestMatchHits {
				bestMatchHits = hits
				bestMatch = old
			}
		}
		if bestMatch != nil {
			keysLengthEqual := len(fresh.Keys) == len(bestMatch.Keys)
			regressionInSameDirection := math.Signbit(fresh.StepFit.Regression) == math.Signbit(bestMatch.StepFit.Regression)
			freshHasBetterFit := math.Abs(fresh.StepFit.Regression) > math.Abs(bestMatch.StepFit.Regression)
			freshHasMoreKeys := len(fresh.Keys) > len(bestMatch.Keys)
			if freshHasMoreKeys || (keysLengthEqual && regressionInSameDirection && freshHasBetterFit) {
				fresh.Status = bestMatch.Status
				fresh.Message = bestMatch.Message
				fresh.ID = bestMatch.ID
				fresh.Bugs = bestMatch.Bugs
				ret = append(ret, fresh)
				// Find the bestMatch in oldSummaries and replace it with fresh.
				for i, oldBest := range oldSummaries {
					if oldBest == bestMatch {
						oldSummaries[i] = fresh
						break
					}
				}
			}
		} else {
			ret = append(ret, fresh)
		}
	}
	return ret
}

// processRows reads all the rows from the clusters table and constructs a
// slice of ClusterSummary's from them.
func processRows(rows *sql.Rows, err error) ([]*types.ClusterSummary, error) {
	if err != nil {
		return nil, fmt.Errorf("Failed to read from database: %s", err)
	}
	defer rows.Close()

	glog.Infof("Found rows %v", rows)

	ret := []*types.ClusterSummary{}

	for rows.Next() {
		var body string
		var id int64
		if err := rows.Scan(&id, &body); err != nil {
			return nil, fmt.Errorf("Failed to read row from database: %s", err)
		}
		c := &types.ClusterSummary{}
		if err := json.Unmarshal([]byte(body), c); err != nil {
			glog.Errorf("Found invalid JSON in clusters table: %s %s", id, err)
			return nil, fmt.Errorf("Failed to read row from database: %s", err)
		}
		c.ID = id
		glog.Infof("ID: %d", id)
		ret = append(ret, c)
	}

	return ret, nil
}

// ListFrom returns all clusters that have a step that occur after the given
// timestamp.
func ListFrom(ts int64) ([]*types.ClusterSummary, error) {
	rows, err := db.DB.Query("SELECT id, cluster FROM clusters WHERE ts>=? ORDER BY status DESC, ts DESC", ts)
	return processRows(rows, err)
}

// ListByStatus returns all clusters that match the given status.
func ListByStatus(status string) ([]*types.ClusterSummary, error) {
	rows, err := db.DB.Query("SELECT id, cluster FROM clusters WHERE status=?", status)
	return processRows(rows, err)
}

// Get returns the cluster that matches the given id.
func Get(id int64) (*types.ClusterSummary, error) {
	rows, err := db.DB.Query("SELECT id, cluster FROM clusters WHERE id=?", id)
	matches, err := processRows(rows, err)
	if len(matches) == 0 {
		return nil, fmt.Errorf("Failed to find cluster summary with id: %d", id)
	}
	return matches[0], nil
}

// Write writes a ClusterSummary to the datastore.
//
// If the ID is set to -1 then write it as a new entry, otherwise update the
// existing entry.
func Write(c *types.ClusterSummary) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("Failed to encode to JSON: %s", err)
	}
	if c.ID == -1 {
		_, err := db.DB.Exec(
			"INSERT INTO clusters (ts, hash, regression, cluster, status, message) VALUES (?, ?, ?, ?, ?, ?)",
			c.Timestamp, c.Hash, c.StepFit.Regression, string(b), c.Status, c.Message)
		if err != nil {
			return fmt.Errorf("Failed to write to database: %s", err)
		}
	} else {
		_, err := db.DB.Exec(
			"UPDATE clusters SET ts=?, hash=?, regression=?, cluster=?, status=?, message=? WHERE id=?",
			c.Timestamp, c.Hash, c.StepFit.Regression, string(b), c.Status, c.Message, c.ID)
		if err != nil {
			return fmt.Errorf("Failed to update database: %s", err)
		}
	}
	return nil
}

// Reset removes all non-Bug alerts from the database.
func Reset() error {
	_, err := db.DB.Exec("DELETE FROM clusters WHERE status!='Bug'")
	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

func skpOnly(_ string, tr *types.PerfTrace) bool {
	return tr.Params()["source_type"] == "skp"
}

// apiKeyFromFlag returns the key that it was passed if the key isn't empty,
// otherwise it tries to fetch the key from the metadata server.
//
// Returns the API Key, or "" if it failed to fetch the key from the metadata
// server.
func apiKeyFromFlag(apiKeyFlag string) string {
	apiKey := apiKeyFlag
	// If apiKey isn't passed in then read it from the metadata server.
	if apiKey == "" {
		var err error
		if apiKey, err = metadata.Get(APIKEY_METADATA_KEY); err != nil {
			glog.Errorf("Retrieving API Key failed: %s", err)
			return ""
		}
	}
	return apiKey
}

// Issue is an individual issue returned from the project hosting response.
//
// It is used in IssueResponse.
type Issue struct {
	ID int64 `json:"id"`
}

// IssueResponse is used to decode JSON responses from the project hosting API.
type IssueResponse struct {
	Items []*Issue `json:"items"`
}

// updateBugs will find all the bugs the reference the alerting cluster will
// write them into the ClusterSummary and save it back to the store.
func updateBugs(c *types.ClusterSummary, apiKey string) {
	// All issues reported through skiaperf will contain a URL of the form:
	//
	//   http://skiaperf.com/cl/NNN.
	//
	// Where NNN is the alerting cluster ID.

	// Search through the project hosting API for all issues that match that URI.
	url := "https://www.googleapis.com/projecthosting/v2/projects/skia/issues?q=%3A%2F%2Fskiaperf.com%2Fcl%2F" + strconv.Itoa(int(c.ID)) + ".&fields=items%2Fid,items%2Fstate&key=" + apiKey

	//  This will return a JSON response of the form:
	//
	//  {
	//   "items": [
	//    {
	//     "id": 2874,
	//     "state": "open"
	//    }
	//   ]
	//  }
	//
	// We don't currently use "state".

	resp, err := http.Get(url)
	if err != nil {
		glog.Errorf("Request to project hosting failed: %s", err)
		return
	}
	defer resp.Body.Close()

	issueResponse := &IssueResponse{
		Items: []*Issue{},
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&issueResponse); err != nil {
		glog.Errorf("Failed to decode project hosting response: %s", err)
		return
	}
	glog.Infof("For %d Got %#v", c.ID, issueResponse)
	bugs := []int64{}
	for _, issue := range issueResponse.Items {
		bugs = append(bugs, issue.ID)
	}
	if !util.Int64Equal(bugs, c.Bugs) {
		c.Bugs = bugs
		if err := Write(c); err != nil {
			glog.Errorf("Alerting: Failed to write updated cluster with bugs: %s", err)
		}
	}
}

// Start kicks off a go routine the periodically refreshes the current alerting clusters.
func Start(tileStore types.TileStore, apiKeyFlag string) {

	apiKey := apiKeyFromFlag(apiKeyFlag)

	// The number of clusters with a status of "New".
	newClustersGauge := metrics.NewRegisteredGauge("alerting.new", metrics.DefaultRegistry)

	// The number of times we've successfully done alert clustering.
	runsCounter := metrics.NewRegisteredCounter("alerting.runs", metrics.DefaultRegistry)

	// How long it takes to do a clustering run.
	alertingLatency := metrics.NewRegisteredTimer("alerting.latency", metrics.DefaultRegistry)

	go func() {
		for _ = range time.Tick(config.RECLUSTER_DURATION) {
			begin := time.Now()
			tile, err := tileStore.Get(0, -1)
			if err != nil {
				glog.Errorf("Alerting: Failed to get tile: %s", err)
				continue
			}

			summary, err := clustering.CalculateClusterSummaries(tile, CLUSTER_SIZE, CLUSTER_STDDEV, skpOnly)
			if err != nil {
				glog.Errorf("Alerting: Failed to calculate clusters: %s", err)
				continue
			}
			fresh := []*types.ClusterSummary{}
			for _, c := range summary.Clusters {
				if math.Abs(c.StepFit.Regression) > clustering.INTERESTING_THRESHHOLD {
					fresh = append(fresh, c)
				}
			}
			old, err := ListFrom(tile.Commits[0].CommitTime)
			if err != nil {
				glog.Errorf("Alerting: Failed to get existing clusters: %s", err)
				continue
			}
			glog.Infof("Found %d old", len(old))
			glog.Infof("Found %d fresh", len(fresh))
			updated := CombineClusters(fresh, old)
			for _, c := range updated {
				if c.Status == "" {
					c.Status = "New"
				}
				if err := Write(c); err != nil {
					glog.Errorf("Alerting: Failed to write updated cluster: %s", err)
				}
			}

			current, err := ListFrom(tile.Commits[0].CommitTime)
			if err != nil {
				glog.Errorf("Alerting: Failed to get existing clusters: %s", err)
				continue
			}
			count := 0
			for _, c := range current {
				if c.Status == "New" {
					count++
				}
				if apiKey != "" {
					updateBugs(c, apiKey)
				} else {
					glog.Infof("Skipping ClusterSummary.Bugs update because apiKey is missing.")
					continue
				}
			}
			newClustersGauge.Update(int64(count))
			runsCounter.Inc(1)
			alertingLatency.UpdateSince(begin)

			// TODO Now do a search of the issue tracker for related bugs for each cluster.
			// Search for links in bugs to each cluster.
			// Extract and add to the Cluster, write the cluster back if changed.
		}
	}()
}
