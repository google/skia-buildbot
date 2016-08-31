// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package jobsim

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/data/rand/cryptorand"
	"github.com/luci/luci-go/common/lhttp"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/dm/api/distributor/jobsim"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/grpc/prpc"
	authlib "github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"
)

// state is the opaque state data that DM will pass between re-executions of the
// same Attempt. For jobsim, which just does accumulating math calculations,
// this state is very simple. For a recipe, it could be more complicated, and on
// swarming, 'state' could be e.g. ISOLATED_OUTDIR's isolate hash.
//
// Generically, this COULD be used to cache e.g. all previous dependencies, but
// it's generally better to cache the result of consuming those dependencies,
// if possible (to avoid doing unnecessary work on subsequent executions).
type state struct {
	Stage int
	Sum   int64
}

func (st *state) toPersistentState() string {
	ret, err := json.Marshal(st)
	if err != nil {
		panic(err)
	}
	return string(ret)
}

func (st *state) fromPersistentState(s string) (err error) {
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), st)
}

// runner holds the context for a single execution of the JobSim task. You could
// think of this as the recipe engine. Everything that the runner does would
// need to be done by any DM client during a single Execution in some form or
// another.
type runner struct {
	c context.Context

	// auth contains the current Execution_Auth for this Execution. It begins with
	// Token == activation token (what DM submits to swarming), which is then
	// transmuted by activateExecution into an execution token (basically, the
	// Execution becomes active on it's first contact with DM, which prevents
	// accidental retries/duplication of Execution work. Retries of failed
	// Executions need to be handled by DM for this reason).
	auth *dm.Execution_Auth

	// dmc is just a DM pRPC client
	dmc dm.DepsClient

	// state is the mutable state for this Execution. It begins with the value of
	// the previous Execution. When this execution finishes, the new value will be
	// passed to the next Execution.
	state *state
}

// getRandom returns a random byte sequence of `want` length, or an error if
// that number of random bytes could not be generated.
//
// This uses cryptorand to generate cryptographically secure random numbers.
func getRandom(c context.Context, want int) ([]byte, error) {
	ret := make([]byte, want)
	_, err := cryptorand.Read(c, ret)
	if err != nil {
		logging.WithError(err).Errorf(c, "could not generate random data")
		return nil, err
	}
	return ret, nil
}

// activateExecution is essentially a direct map to ActivateExecution, except
// that it transforms the runner's `auth` from a needs-activation authentication
// to an activated one.
//
// This returns an OK bool, plus an error. If the bool is true, it means that
// we're now authenticated. This will never return true with a non-nil error.
//
// The return value (false, nil) means the runner should quit peacefully
// (no-error), and can occur if this execution payload is retried (e.g. by
// taskqueue).
func (r *runner) activateExecution() (ok bool, err error) {
	newToken, err := getRandom(r.c, 128)
	if err != nil {
		return false, err
	}

	_, err = r.dmc.ActivateExecution(r.c, &dm.ActivateExecutionReq{
		Auth:           r.auth,
		ExecutionToken: newToken,
	})
	if err != nil {
		logging.WithError(err).Errorf(r.c, "could not activate execution")
		if grpcutil.Code(err) == codes.Unauthenticated {
			// means we got retried, so invalidate ExecutionKey and quit peacefully.
			return false, nil
		}
		return false, err
	}
	r.auth.Token = newToken
	logging.Fields{"token": r.auth.Token}.Infof(r.c, "activated execution")
	return true, nil
}

// doReturnStage implements the FinishAttempt portion of the DM Attempt
// lifecycle. This sets the final result for the Attempt, and will prevent any
// further executions of this Attempt. This is analogous to the recipe returning
// a final result.
func (r *runner) doReturnStage(stg *jobsim.ReturnStage) error {
	retval := int64(0)
	if stg != nil {
		retval += stg.Retval
	}
	if r.state != nil {
		retval += r.state.Sum
	}

	_, err := r.dmc.FinishAttempt(r.c, &dm.FinishAttemptReq{
		Auth: r.auth,
		Data: executionResult(true, retval, stg.GetExpiration().Time()),
	})
	if err != nil {
		logging.WithError(err).Warningf(r.c, "got error on FinishAttempt")
	}
	return err
}

// doDeps will call EnsureGraphData to make sure the provided
// quests+attempts+deps exist. DM will see if they're done or not, and let us
// know if we should stop to be retried later.
func (r *runner) doDeps(seed int64, stg *jobsim.DepsStage, cfgName string) (stop bool, err error) {
	logging.Infof(r.c, "doing deps")

	stg.ExpandShards()

	req := &dm.EnsureGraphDataReq{}
	req.ForExecution = r.auth
	req.Include = &dm.EnsureGraphDataReq_Include{
		Attempt: &dm.EnsureGraphDataReq_Include_Options{Result: true}}

	var (
		rnd = rand.New(rand.NewSource(seed))

		maxAttemptNum  = map[string]uint32{}
		currentAttempt = map[string]uint32{}
	)

	for i, dep := range stg.Deps {
		dep.Seed(rnd, seed, int64(i))

		desc := &dm.Quest_Desc{}
		desc.DistributorConfigName = cfgName
		desc.Parameters, err = (&jsonpb.Marshaler{}).MarshalToString(dep.Phrase)
		if err != nil {
			panic(err)
		}

		if err := desc.Normalize(); err != nil {
			panic(err)
		}

		req.Quest = append(req.Quest, desc)

		qid := desc.QuestID()
		currentAttempt[qid] = 1
		switch x := dep.AttemptStrategy.(type) {
		case *jobsim.Dependency_Attempts:
			req.QuestAttempt = append(req.QuestAttempt, &dm.AttemptList_Nums{Nums: x.Attempts.ToSlice()})
		case *jobsim.Dependency_Retries:
			maxAttemptNum[qid] = x.Retries + 1
			req.QuestAttempt = append(req.QuestAttempt, &dm.AttemptList_Nums{Nums: []uint32{1}})
		}
	}

	rsp, err := r.dmc.EnsureGraphData(r.c, req)
	if err != nil {
		logging.Fields{"err": err}.Infof(r.c, "error after first rpc")
		return
	}
	if stop = rsp.ShouldHalt; stop {
		logging.Infof(r.c, "halt after first rpc")
		return
	}

	sum := int64(0)
	for {
		req.Quest = nil
		req.QuestAttempt = nil
		req.RawAttempts = dm.NewAttemptList(nil)

		// TODO(iannucci): we could use the state api to remember that we did
		// retries on the previous execution. The recipe engine should probably do
		// this for recipes, but for this simple simulator, we'll make multiple
		// RPCs.
		for qid, q := range rsp.Result.Quests {
			logging.Fields{"qid": qid}.Infof(r.c, "grabbing result")
			var tr *jobsim.Result
			if currentAttempt[qid] == 0 {
				// rsp contains bonus data
				continue
			}
			logging.Fields{"atmpt": q.Attempts[currentAttempt[qid]].Data}.Infof(r.c, "grabbing payload")
			payload := q.Attempts[currentAttempt[qid]].Data.GetFinished().Data
			tr, err = executionResultFromJSON(payload)
			if err != nil {
				return
			}
			logging.Fields{"qid": qid, "tr": tr}.Infof(r.c, "decoded TaskResult")
			if tr.Success {
				sum += tr.Value
			} else {
				current := currentAttempt[qid]
				if maxAttempt := maxAttemptNum[qid]; maxAttempt > current {
					if current > maxAttempt {
						logging.Fields{
							"qid": qid, "current": current, "max": maxAttempt,
						}.Infof(r.c, "too many retries")
						return r.doFailure(seed, 1.1) // guarantee failure
					}
					logging.Fields{
						"qid": qid, "current": current, "max": maxAttempt,
					}.Infof(r.c, "retrying")
					current++
					currentAttempt[qid] = current
					req.RawAttempts.To[qid].Nums = []uint32{current}
				} else {
					logging.Fields{
						"qid": qid,
					}.Infof(r.c, "no retries allowed")
					return r.doFailure(seed, 1.1) // guarantee failure
				}
			}
		}
		if req.Quest != nil {
			rsp, err = r.dmc.EnsureGraphData(r.c, req)
			if err != nil {
				logging.Fields{"err": err}.Infof(r.c, "err after Nth rpc")
				return
			}
			if stop = rsp.ShouldHalt; stop {
				logging.Infof(r.c, "halt after Nth rpc")
				return
			}
		} else {
			r.state.Sum += sum
			logging.Fields{"sum": sum}.Infof(r.c, "added and advancing")
			return
		}
	}
}

func (r *runner) doStall(stg *jobsim.StallStage) {
	dur := stg.Delay.Duration()
	logging.Fields{"duration": dur}.Infof(r.c, "stalling")
	clock.Sleep(r.c, dur)
}

// doFailure implements a jobsim task having some flakiness (e.g. it will
// non-deterministically select to fail at this stage). If it fails, this will
// cause the overall application-level Attempt Result to indicate failure to any
// other Attempts which depend on this one.
//
// This is analogous to a recipe running a flaky test and then setting its
// result to be failure. As far as DM is concerned, the recipe ran to completion
// (e.g. FINISHED). Other recipes may decide to take some action at this stage
// (e.g. issue new Attempts of this same Quest).
func (r *runner) doFailure(seed int64, chance float32) (stop bool, err error) {
	failed := chance >= 1.0
	if !failed {
		logging.Fields{"chance": chance}.Infof(r.c, "failing with probability")
		rndVal := rand.New(rand.NewSource(seed)).Float32()
		failed = rndVal <= chance
		logging.Fields{"rndVal": rndVal}.Infof(r.c, "failed")
	} else {
		logging.Infof(r.c, "failed (guaranteed)")
	}

	if !failed {
		logging.Infof(r.c, "passed")
		return
	}

	stop = true
	_, err = r.dmc.FinishAttempt(r.c, &dm.FinishAttemptReq{
		Auth: r.auth,
		Data: executionResult(false, 0, time.Time{}),
	})
	if err != nil {
		logging.WithError(err).Warningf(r.c, "got error on FinishAttempt")
	}
	return
}

// runJob is analogous to a single Execution of a recipe. It will:
//   * Activate itself with DM.
//   * Inspect its previous State to determine where it left off on the previous
//     execution.
//   * Execute stages (incrementing the Stage counter in the state, and/or
//     accumulating into Sum) until it hits a stop condition:
//     * depending on incomplete Attempts
//     * arriving at a final result
//
// If it hits some underlying error it will return that error, and expect to be
// retried by DM.
func runJob(c context.Context, host string, state *state, job *jobsim.Phrase, auth *dm.Execution_Auth, cfgName string) error {
	tr, err := authlib.GetRPCTransport(c, authlib.NoAuth)
	if err != nil {
		return err
	}
	pcli := &prpc.Client{
		C:       &http.Client{Transport: tr},
		Host:    host,
		Options: prpc.DefaultOptions(),
	}
	pcli.Options.Insecure = lhttp.IsLocalHost(host)
	dmc := dm.NewDepsPRPCClient(pcli)
	r := runner{c, auth, dmc, state}

	ok, err := r.activateExecution()
	if !ok || err != nil {
		return err
	}

	stop := false
	for ; r.state.Stage < len(job.Stages); r.state.Stage++ {
		switch stg := job.Stages[r.state.Stage].StageType.(type) {
		case *jobsim.Stage_Deps:
			stop, err = r.doDeps(job.Seed, stg.Deps, cfgName)
		case *jobsim.Stage_Stall:
			r.doStall(stg.Stall)
		case *jobsim.Stage_Failure:
			stop, err = r.doFailure(job.Seed, stg.Failure.Chance)
		default:
			err = fmt.Errorf("don't know how to handle StageType: %T", stg)
		}
		if stop || err != nil {
			return err
		}
	}
	return r.doReturnStage(job.ReturnStage)
}
