package dryrun

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
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
	Finished    bool                     `json:"finished"`
	Message     string                   `json:"message"`
	Regressions []*regression.Regression `json:"regressions"`
}

type Requests struct {
	cidl           *cid.CommitIDLookup
	dfBuilder      dataframe.DataFrameBuilder
	paramsProvider regression.ParamsetProvider // TODO build the paramset from dfBuilder.
	mutex          sync.Mutex
	inFlight       map[string]Running
}

func New(cidl *cid.CommitIDLookup, dfBuilder dataframe.DataFrameBuilder, paramsProvider regression.ParamsetProvider) *Requests {
	ret := &Requests{
		cidl:           cidl,
		dfBuilder:      dfBuilder,
		paramsProvider: paramsProvider,
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
	if p, err := d.inFlight[id]; ok {
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
			Regressions: []*regression.Regression{},
		}
		go func() {
			cb := func(clusterResponse []*ClusterResponse, cfg *alerts.Config, q string) {
				running.mutex.Lock()
				defer running.mutex.Unlock()
				// loop over clusterResponse, convert each to regressions, and merge with running.Regressions.
			}
			regression.RegressionsForAlert(context.Background(), req.Config, d.paramsProvider(), cb, req.Domain.NumCommits, req.Domain.End, d.git, d.cidl, d.dfBuilder)
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

func (d *Requests) StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	var status StatusResponse
	d.mutex.Lock()
	defer d.mutex.Unlock()

	running, ok := d.inFlight[id]
	if !ok {
		httputils.ReportError(w, r, fmt.Errorf("Invalid id: %q", id), "Invalid or expired dry run.")
		return
	}
	running.mutex.Lock()
	defer running.mutex.Unlock()
	if err := json.NewEncoder(w).Encode(running); err != nil {
		sklog.Errorf("Failed to encode paramset: %s", err)
	}
}
