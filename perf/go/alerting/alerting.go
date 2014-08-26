package alerting

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"

	"skia.googlesource.com/buildbot.git/perf/go/clustering"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/db"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

// CombineClusters combines freshly found clusters with existing clusters.
//
//  Algorithm:
//    Run clustering and pick out the "Interesting" clusters.
//    Compare all the Interestin clusters to all the existing relevant clusters,
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
//        Take the better of the two clusters (old/new) based on the better
//        Regression score, i.e. larger |Regression|, and update that in the "list".
//    Save all the clusters in the "list" back to the db.
func CombineClusters(freshSummaries, oldSummaries []*types.ClusterSummary) []*types.ClusterSummary {
	ret := []*types.ClusterSummary{}

	for _, fresh := range freshSummaries {
		var bestMatch *types.ClusterSummary = nil
		bestMatchHits := 0
		for _, old := range oldSummaries {
			numKeys := 20
			if len(old.Keys) < numKeys {
				numKeys = len(old.Keys)
			}
			hits := 0
			for _, key := range old.Keys[:numKeys] {
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
			if math.Abs(fresh.StepFit.Regression) > math.Abs(bestMatch.StepFit.Regression) {
				fresh.Status = bestMatch.Status
				fresh.Message = bestMatch.Message
				fresh.ID = bestMatch.ID
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
	rows, err := db.DB.Query("SELECT id, cluster FROM clusters WHERE ts>=? ORDER BY status DESC", ts)
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

func skpOnly(tr *types.Trace) bool {
	return tr.Params["source_type"] == "skp"
}

// Start kicks off a go routine the periodically refreshes the current alerting clusters.
func Start(tileStore types.TileStore) {

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

			summary, err := clustering.CalculateClusterSummaries(tile, 50, 0.1, skpOnly)
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
			}
			newClustersGauge.Update(int64(count))
			runsCounter.Inc(1)
			alertingLatency.UpdateSince(begin)

			// TODO Now do a search of the issue tracker for related bugs for each cluster.
			// Search for links in bugs to each cluster.
			// Extract and add to the Cluster, write the cluster back if changed.
			// "https://www.googleapis.com/projecthosting/v2/projects/skia/issues?q=http%3A%2F%2Fskiaperf.com%2Fcluster%2F&fields=items%2Fid&key=
		}
	}()
}
