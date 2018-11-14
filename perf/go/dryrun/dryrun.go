package dryrun

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
)

func main() {
	fmt.Println("vim-go")
}

type Domain struct {
	Begin       int                   `json:"begin"`       // Beginning of time range in Unix timestamp seconds.
	End         int                   `json:"end"`         // End of time range in Unix timestamp seconds.
	NumCommits  int32                 `json:"num_commits"` // If RequestType is REQUEST_COMPACT, then the number of commits to show before End, and Begin is ignored.
	RequestType dataframe.RequestType `json:"request_type"`
}

type StartRequest struct {
	Config alerts.Config `json:"config"`
	Domain Domain        `json:"domain"`
}

func (s *StartRequest) Id() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%#v", *s))))
}

type StartResponse struct {
	ID string `json:"id"`
}

type Running struct {
	mutex       sync.Mutex
	Finished    bool                              `json:"finished"`
	Message     string                            `json:"message"`
	Regressions map[string]*regression.Regression `json:"regressions"`
}

type Requests struct {
	cidl           *cid.CommitIDLookup
	dfBuilder      dataframe.DataFrameBuilder
	git            *gitinfo.GitInfo
	paramsProvider regression.ParamsetProvider // TODO build the paramset from dfBuilder.
	mutex          sync.Mutex
	inFlight       map[string]*Running
}

func New(cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder, paramsProvider regression.ParamsetProvider, git *gitinfo.GitInfo) *Requests {
	ret := &Requests{
		cidl:           cidl,
		dfBuilder:      dfBuilder,
		paramsProvider: paramsProvider,
		git:            git,
		inFlight:       map[string]*Running{},
	}
	// Start a process to clean up old responses.
	return ret
}

func (d *Requests) StartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, r, err, "Could not decode POST body.")
		return
	}
	d.mutex.Lock()
	defer d.mutex.Unlock()
	id := req.Id()
	if p, ok := d.inFlight[id]; ok {
		p.mutex.Lock()
		defer p.mutex.Unlock()
		if p.Finished {
			delete(d.inFlight, id)
		}
	}
	if _, ok := d.inFlight[id]; !ok {
		running := &Running{
			Finished:    false,
			Message:     "",
			Regressions: map[string]*regression.Regression{},
		}
		d.inFlight[id] = running
		go func() {
			ctx := context.Background()
			cb := func(clusterResponse []*regression.ClusterResponse) {
				running.mutex.Lock()
				defer running.mutex.Unlock()
				// loop over clusterResponse, convert each to regressions, and merge with running.Regressions.
				for _, cr := range clusterResponse {
					c, reg, err := regression.RegressionFromClusterResponse(ctx, cr, &req.Config, d.cidl)
					if err != nil {
						running.Message = "Failed to convert to Regression, some data may be missing."
						sklog.Errorf("Failed to convert to Regression: %s", err)
						return
					}
					id := c.ID()
					if origReg, ok := running.Regressions[id]; !ok {
						running.Regressions[id] = reg
					} else {
						running.Regressions[id] = origReg.Merge(reg)
					}
				}
			}
			end := time.Unix(int64(req.Domain.End), 0)
			regression.RegressionsForAlert(ctx, &req.Config, d.paramsProvider(), cb, int(req.Domain.NumCommits), end, d.git, d.cidl, d.dfBuilder)
			running.mutex.Lock()
			defer running.mutex.Unlock()
			running.Finished = true
		}()
	}
	resp := StartResponse{
		ID: id,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}

type RegressionRow struct {
	CID        *cid.CommitDetail      `json:"cid"`
	Regression *regression.Regression `json:"regression"`
}

type Status struct {
	Finished    bool             `json:"finished"`
	Message     string           `json:"message"`
	Regressions []*RegressionRow `json:"regressions"`
}

func (d *Requests) StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	d.mutex.Lock()
	defer d.mutex.Unlock()

	running, ok := d.inFlight[id]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("Invalid id: %q", id), "Invalid or expired dry run.")
		return
	}
	running.mutex.Lock()
	defer running.mutex.Unlock()
	keys := []string{}
	for id, _ := range running.Regressions {
		keys = append(keys, id)
	}
	sort.Strings(keys)

	cids := []*cid.CommitID{}
	for _, key := range keys {
		commitId, err := cid.FromID(key)
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to parse commit id.")
			return
		}
		cids = append(cids, commitId)
	}

	cidd, err := d.cidl.Lookup(r.Context(), cids)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to find commit ids.")
		return
	}
	status := &Status{
		Finished:    running.Finished,
		Message:     running.Message,
		Regressions: []*RegressionRow{},
	}
	for _, details := range cidd {
		status.Regressions = append(status.Regressions, &RegressionRow{
			CID:        details,
			Regression: running.Regressions[details.ID()],
		})

	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}
