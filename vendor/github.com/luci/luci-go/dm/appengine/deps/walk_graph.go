// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deps

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/luci/gae/filter/dsQueryBatch"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/sync/parallel"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/grpc/grpcutil"
)

const numWorkers = 16

const maxTimeout = 55 * time.Second // GAE limit is 60s

type node struct {
	aid                 *dm.Attempt_ID
	depth               int64
	canSeeAttemptResult bool
}

func (g *graphWalker) runSearchQuery(send func(*dm.Attempt_ID) error, s *dm.GraphQuery_Search) func() error {
	return func() error {
		logging.Errorf(g, "SearchQuery not implemented")
		return grpcutil.Errf(codes.Unimplemented, "GraphQuery.Search is not implemented")
	}
}

func isCtxErr(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}

func (g *graphWalker) runAttemptListQuery(send func(*dm.Attempt_ID) error) func() error {
	return func() error {
		for qst, anum := range g.req.Query.AttemptList.To {
			if len(anum.Nums) == 0 {
				qry := model.QueryAttemptsForQuest(g, qst)
				if !g.req.Include.Attempt.Abnormal {
					qry = qry.Eq("IsAbnormal", false)
				}
				if !g.req.Include.Attempt.Expired {
					qry = qry.Eq("IsExpired", false)
				}
				err := datastore.Get(g).Run(qry, func(k *datastore.Key) error {
					aid := &dm.Attempt_ID{}
					if err := aid.SetDMEncoded(k.StringID()); err != nil {
						logging.WithError(err).Errorf(g, "Attempt_ID.SetDMEncoded returned an error with input: %q", k.StringID())
						panic(fmt.Errorf("in AttemptListQuery: %s", err))
					}
					return send(aid)
				})
				if err != nil {
					if !isCtxErr(err) {
						logging.WithError(err).Errorf(g, "in AttemptListQuery")
					}
					return err
				}
			} else {
				for _, num := range anum.Nums {
					if err := send(dm.NewAttemptID(qst, num)); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
}

func (g *graphWalker) runAttemptRangeQuery(send func(*dm.Attempt_ID) error, ar *dm.GraphQuery_AttemptRange) func() error {
	return func() error {
		for i := ar.Low; i < ar.High; i++ {
			if err := send(dm.NewAttemptID(ar.Quest, i)); err != nil {
				return err
			}
		}
		return nil
	}
}

func (g *graphWalker) questDataLoader(qid string, dst *dm.Quest) func() error {
	return func() error {
		qst := &model.Quest{ID: qid}
		if err := datastore.Get(g).Get(qst); err != nil {
			if err == datastore.ErrNoSuchEntity {
				dst.DNE = true
				dst.Partial = false
				return nil
			}
			logging.Fields{ek: err, "qid": qid}.Errorf(g, "while loading quest")
			return err
		}
		dst.Data = qst.DataProto()
		dst.Partial = false
		return nil
	}
}

func (g *graphWalker) loadEdges(send func(*dm.Attempt_ID) error, typ string, base *datastore.Key, fan *dm.AttemptList, doSend bool) error {
	return datastore.Get(g).Run(datastore.NewQuery(typ).Ancestor(base), func(k *datastore.Key) error {
		if g.Err() != nil {
			return datastore.Stop
		}
		aid := &dm.Attempt_ID{}
		if err := aid.SetDMEncoded(k.StringID()); err != nil {
			logging.WithError(err).Errorf(g, "Attempt_ID.SetDMEncoded returned an error with input: %q", k.StringID())
			panic(fmt.Errorf("in AttemptListQuery: %s", err))
		}
		if fan != nil {
			fan.AddAIDs(aid)
		}
		if doSend {
			if err := send(aid); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *graphWalker) loadExecutions(includeResult bool, atmpt *model.Attempt, akey *datastore.Key, dst *dm.Attempt) error {
	numEx := g.req.Include.NumExecutions
	if atmpt.CurExecution < numEx {
		numEx = atmpt.CurExecution
	}

	dst.Executions = map[uint32]*dm.Execution{}
	ds := datastore.Get(g)
	q := datastore.NewQuery("Execution").Ancestor(akey)
	if numEx > math.MaxInt32 {
		q = q.Limit(math.MaxInt32)
	} else {
		q = q.Limit(int32(numEx))
	}
	if !g.req.Include.Execution.Abnormal {
		q = q.Eq("IsAbnormal", false)
	}
	if !g.req.Include.Execution.Expired {
		q = q.Eq("IsExpired", false)
	}

	return ds.Run(q, func(e *model.Execution) error {
		if g.Err() != nil {
			return datastore.Stop
		}
		p := e.ToProto(g.req.Include.Execution.Ids)
		d, err := g.getDist(p.Data.DistributorInfo.ConfigName)
		if err != nil {
			return err
		}
		p.Data.DistributorInfo.Url = d.InfoURL(distributor.Token(p.Data.DistributorInfo.Token))
		if d := p.Data.GetFinished().GetData(); d != nil {
			if !includeResult || !g.lim.Add(d.Size) {
				d.Object = ""
			}
		}
		dst.Executions[uint32(e.ID)] = p
		return nil
	})
}

func (g *graphWalker) attemptResultLoader(aid *dm.Attempt_ID, rsltSize uint32, akey *datastore.Key, dst *dm.Attempt) func() error {
	return func() error {
		ds := datastore.Get(g)

		siz := rsltSize
		if !g.lim.PossiblyOK(siz) {
			dst.Partial.Result = dm.Attempt_Partial_DATA_SIZE_LIMIT
			logging.Infof(g, "skipping load of AttemptResult %s (size limit)", aid)
			return nil
		}
		r := &model.AttemptResult{Attempt: akey}
		if err := ds.Get(r); err != nil {
			logging.Fields{ek: err, "aid": aid}.Errorf(g, "failed to load AttemptResult")
			return err
		}
		if g.lim.Add(siz) {
			dst.Data.GetFinished().Data = &r.Data
			dst.Partial.Result = dm.Attempt_Partial_LOADED
		} else {
			dst.Partial.Result = dm.Attempt_Partial_DATA_SIZE_LIMIT
			logging.Infof(g, "loaded AttemptResult %s, but hit size limit after", aid)
		}
		return nil
	}
}

func (g *graphWalker) checkCanLoadResultData(aid *dm.Attempt_ID) (bool, error) {
	// We're running this query from a human, or internally in DM, not a bot.
	if g.req.Auth == nil {
		return true, nil
	}

	// This query is from a bot, and we didn't get to this Attempt by walking
	// a dependency, so we need to verify that the dependency exists.
	from := g.req.Auth.Id.AttemptID().DMEncoded()
	to := aid.DMEncoded()
	ds := datastore.Get(g)
	fdepKey := ds.MakeKey("Attempt", from, "FwdDep", to)
	exist, err := ds.Exists(fdepKey)
	if err != nil {
		logging.Fields{ek: err, "key": fdepKey}.Errorf(g, "failed to determine if FwdDep exists")
		return false, err
	}
	if !exist.All() {
		return false, nil
	}

	return true, nil
}

func (g *graphWalker) attemptLoader(aid *dm.Attempt_ID, authedForResult bool, dst *dm.Attempt, send func(aid *dm.Attempt_ID, fwd bool) error) func() error {
	return func() error {
		ds := datastore.Get(g)

		atmpt := &model.Attempt{ID: *aid}
		akey := ds.KeyForObj(atmpt)
		if err := ds.Get(atmpt); err != nil {
			if err == datastore.ErrNoSuchEntity {
				dst.DNE = true
				dst.Partial = nil
				return nil
			}
			return err
		}
		if !g.req.Include.Attempt.Abnormal && atmpt.IsAbnormal {
			return nil
		}
		if !g.req.Include.Attempt.Expired && atmpt.IsExpired {
			return nil
		}

		if g.req.Include.Attempt.Data {
			dst.Data = atmpt.DataProto()
			dst.Partial.Data = false
		}

		canLoadResultData := false
		if g.req.Include.Attempt.Result {
			// We loaded this by walking from the Attempt, so we don't need to do any
			// additional checks.
			if authedForResult {
				canLoadResultData = true
			} else {
				canLoad, err := g.checkCanLoadResultData(aid)
				if !canLoad {
					dst.Partial.Result = dm.Attempt_Partial_NOT_AUTHORIZED
				}
				if err != nil {
					return err
				}
				canLoadResultData = canLoad
			}
		}

		errChan := parallel.Run(0, func(ch chan<- func() error) {
			if g.req.Include.Attempt.Result {
				if atmpt.State == dm.Attempt_FINISHED {
					if canLoadResultData {
						ch <- g.attemptResultLoader(aid, atmpt.Result.Data.Size, akey, dst)
					}
				} else {
					dst.Partial.Result = dm.Attempt_Partial_LOADED
				}
			}

			if g.req.Include.NumExecutions > 0 {
				ch <- func() error {
					err := g.loadExecutions(canLoadResultData, atmpt, akey, dst)
					if err != nil {
						logging.Fields{ek: err, "aid": aid}.Errorf(g, "error loading executions")
					} else {
						dst.Partial.Executions = false
					}
					return err
				}
			}

			writeFwd := g.req.Include.FwdDeps
			walkFwd := (g.req.Mode.Direction == dm.WalkGraphReq_Mode_BOTH ||
				g.req.Mode.Direction == dm.WalkGraphReq_Mode_FORWARDS)
			loadFwd := writeFwd || walkFwd

			if loadFwd {
				isAuthed := g.req.Auth != nil && g.req.Auth.Id.AttemptID().Equals(aid)
				subSend := func(aid *dm.Attempt_ID) error {
					return send(aid, isAuthed)
				}
				if writeFwd {
					dst.FwdDeps = dm.NewAttemptList(nil)
				}
				ch <- func() error {
					err := g.loadEdges(subSend, "FwdDep", akey, dst.FwdDeps, walkFwd)
					if err == nil && dst.Partial != nil {
						dst.Partial.FwdDeps = false
					} else if err != nil {
						logging.Fields{"aid": aid, ek: err}.Errorf(g, "while loading FwdDeps")
					}
					return err
				}
			}

			writeBack := g.req.Include.BackDeps
			walkBack := (g.req.Mode.Direction == dm.WalkGraphReq_Mode_BOTH ||
				g.req.Mode.Direction == dm.WalkGraphReq_Mode_BACKWARDS)
			loadBack := writeBack || walkBack

			if loadBack {
				if writeBack {
					dst.BackDeps = dm.NewAttemptList(nil)
				}
				subSend := func(aid *dm.Attempt_ID) error {
					return send(aid, false)
				}
				bdg := &model.BackDepGroup{Dependee: atmpt.ID}
				bdgKey := ds.KeyForObj(bdg)
				ch <- func() error {
					err := g.loadEdges(subSend, "BackDep", bdgKey, dst.BackDeps, walkBack)
					if err == nil && dst.Partial != nil {
						dst.Partial.BackDeps = false
					} else if err != nil {
						logging.Fields{"aid": aid, ek: err}.Errorf(g, "while loading FwdDeps")
					}
					return err
				}
			}
		})
		hadErr := false
		for err := range errChan {
			if err == nil {
				continue
			}
			if err != nil {
				hadErr = true
			}
		}
		dst.NormalizePartial()
		if hadErr {
			return fmt.Errorf("had one or more errors loading attempt")
		}
		return nil
	}
}

type graphWalker struct {
	context.Context
	req *dm.WalkGraphReq

	lim *sizeLimit
	reg distributor.Registry

	distCacheLock sync.Mutex
	distCache     map[string]distributor.D
}

func (g *graphWalker) getDist(cfgName string) (distributor.D, error) {
	g.distCacheLock.Lock()
	defer g.distCacheLock.Unlock()

	dist := g.distCache[cfgName]
	if dist != nil {
		return dist, nil
	}

	dist, _, err := g.reg.MakeDistributor(g, cfgName)
	if err != nil {
		logging.Fields{
			ek: err, "cfgName": cfgName,
		}.Errorf(g, "unable to load distributor")
		return nil, err
	}

	if g.distCache == nil {
		g.distCache = map[string]distributor.D{}
	}
	g.distCache[cfgName] = dist
	return dist, nil
}

func (g *graphWalker) excludedQuest(qid string) bool {
	qsts := g.req.Exclude.Quests
	if len(qsts) == 0 {
		return false
	}
	idx := sort.Search(len(qsts), func(i int) bool { return qsts[i] >= qid })
	return idx < len(qsts) && qsts[idx] == qid
}

func (g *graphWalker) excludedAttempt(aid *dm.Attempt_ID) bool {
	to := g.req.Exclude.Attempts.GetTo()
	if len(to) == 0 {
		return false
	}
	toNums, ok := to[aid.Quest]
	if !ok {
		return false
	}
	nums := toNums.Nums
	if nums == nil { // a nil Nums list means 'ALL nums for this quest'
		return true
	}
	idx := sort.Search(len(nums), func(i int) bool { return nums[i] >= aid.Id })
	return idx < len(nums) && nums[idx] == aid.Id
}

func doGraphWalk(c context.Context, req *dm.WalkGraphReq) (rsp *dm.GraphData, err error) {
	cncl := (func())(nil)
	timeoutProto := req.Limit.MaxTime
	timeout := timeoutProto.Duration() // .Duration on nil is OK
	if timeoutProto == nil || timeout > maxTimeout {
		timeout = maxTimeout
	}
	// we end up doing a lot of long queries in here, so install a batcher to
	// prevent datastore timeouts.
	c = dsQueryBatch.BatchQueries(c, 100)
	c, cncl = clock.WithTimeout(c, timeout)
	defer cncl()

	// nodeChan recieves attempt nodes to process. If it recieves the
	// `finishedJob` sentinel node, that indicates that an outstanding worker is
	// finished.
	nodeChan := make(chan *node, numWorkers)
	defer close(nodeChan)

	g := graphWalker{Context: c, req: req}

	sendNodeAuthed := func(depth int64) func(*dm.Attempt_ID, bool) error {
		return func(aid *dm.Attempt_ID, isAuthed bool) error {
			select {
			case nodeChan <- &node{aid: aid, depth: depth, canSeeAttemptResult: isAuthed}:
				return nil
			case <-c.Done():
				return c.Err()
			}
		}
	}

	sendNode := func(depth int64) func(*dm.Attempt_ID) error {
		return func(aid *dm.Attempt_ID) error {
			select {
			case nodeChan <- &node{aid: aid, depth: depth}:
				return nil
			case <-c.Done():
				return c.Err()
			}
		}
	}

	buf := parallel.Buffer{}
	defer buf.Close()
	buf.Maximum = numWorkers + 1 // +1 job for queries
	buf.Sustained = numWorkers
	buf.SetFIFO(!req.Mode.Dfs)

	allErrs := make(chan error, numWorkers)
	outstandingJobs := 0

	addJob := func(f func() error) {
		if c.Err() != nil { // we're done adding work, ignore this job
			return
		}
		outstandingJobs++
		buf.WorkC() <- parallel.WorkItem{F: f, ErrC: allErrs}
	}

	addJob(func() error {
		return parallel.FanOutIn(func(pool chan<- func() error) {
			snd := sendNode(0)
			if req.Query.AttemptList != nil {
				pool <- g.runAttemptListQuery(snd)
			}
			for _, rng := range req.Query.AttemptRange {
				pool <- g.runAttemptRangeQuery(snd, rng)
			}
			for _, srch := range req.Query.Search {
				pool <- g.runSearchQuery(snd, srch)
			}
		})
	})

	g.lim = &sizeLimit{0, req.Limit.MaxDataSize}
	g.reg = distributor.GetRegistry(c)
	rsp = &dm.GraphData{Quests: map[string]*dm.Quest{}}
	// main graph walk processing loop
	for outstandingJobs > 0 || len(nodeChan) > 0 {
		select {
		case err := <-allErrs:
			outstandingJobs--
			if err == nil {
				break
			}
			if !isCtxErr(err) {
				rsp.HadErrors = true
			}
			// assume that contextualized logging already happened

		case n := <-nodeChan:
			if g.excludedAttempt(n.aid) {
				continue
			}

			qst, ok := rsp.GetQuest(n.aid.Quest)
			if !ok {
				if !req.Include.Quest.Ids {
					qst.Id = nil
				}
				if req.Include.Quest.Data && !g.excludedQuest(n.aid.Quest) {
					addJob(g.questDataLoader(n.aid.Quest, qst))
				}
			}
			if _, ok := qst.Attempts[n.aid.Id]; !ok {
				atmpt := &dm.Attempt{Partial: &dm.Attempt_Partial{
					Data:       req.Include.Attempt.Data,
					Executions: req.Include.NumExecutions != 0,
					FwdDeps:    req.Include.FwdDeps,
					BackDeps:   req.Include.BackDeps,
				}}
				if req.Include.Attempt.Result {
					atmpt.Partial.Result = dm.Attempt_Partial_NOT_LOADED
				}

				atmpt.NormalizePartial() // in case they're all false
				if req.Include.Attempt.Ids {
					atmpt.Id = n.aid
				}
				qst.Attempts[n.aid.Id] = atmpt
				if req.Limit.MaxDepth == -1 || n.depth < req.Limit.MaxDepth {
					addJob(g.attemptLoader(n.aid, n.canSeeAttemptResult, atmpt,
						sendNodeAuthed(n.depth+1)))
				}
			}
			// otherwise, we've dealt with this attempt before, so ignore it.
		}
	}

	if c.Err() != nil {
		rsp.HadMore = true
	}
	return
}

// WalkGraph does all the graph walking in parallel. Theoretically, the
// goroutines look like:
//
//   main:
//     WorkPool(MaxWorkers+1) as pool
//       jobBuf.add(FanOutIn(Query1->nodeChan ... QueryN->nodeChan))
//       runningWorkers = 0
//       while runningWorkers > 0 or jobBuf is not empty:
//         select {
//           pool <- jobBuf.pop(): // only if jobBuf !empty, otherwise skip this send
//             runningWorkers++
//           node = <- nodeChan:
//             if node == finishedJob {
//               runningWorkers--
//             } else {
//               if node mentions new quest:
//                 jobBuf.add(questDataLoader()->nodeChan)
//               if node mentions new attempt:
//                 if node.depth < MaxDepth:
//                   jobBuf.add(attemptLoader(node.depth+1)->nodeChan)
//             }
//         }
//
//   attemptLoader:
//     a = get_attempt()
//     if a exists {
//       FanOutIn(
//         maybeLoadAttemptResult,
//         maybeLoadExecutions,
//         maybeLoadFwdDeps, // sends to nodeChan if walking direction Fwd|Both
//         maybeLoadBackDeps // sends to nodeChan if walking direction Back|Both
//       )
//     }
func (d *deps) WalkGraph(c context.Context, req *dm.WalkGraphReq) (rsp *dm.GraphData, err error) {
	if req.Auth != nil {
		logging.Fields{"execution": req.Auth.Id}.Debugf(c, "on behalf of")
	} else {
		if err = canRead(c); err != nil {
			return
		}
	}
	return doGraphWalk(c, req)
}
