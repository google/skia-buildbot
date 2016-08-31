// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package jobsim

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/jsonpb"

	"github.com/luci/gae/filter/txnBuf"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/taskqueue"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/dm/api/distributor/jobsim"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor"
)

type jobsimDist struct {
	c   context.Context
	cfg *distributor.Config
}

func (j *jobsimDist) jsConfig() *jobsim.Config {
	return j.cfg.Content.(*jobsim.Config)
}

func (j *jobsimDist) parsePayload(payload string) (*jobsimExecution, error) {
	ret := &jobsimExecution{}
	err := jsonpb.UnmarshalString(payload, &ret.Calculation)
	return ret, err
}

func (j *jobsimDist) Run(tsk *distributor.TaskDescription) (tok distributor.Token, _ time.Duration, err error) {
	// TODO(riannucci): Fix luci-gae so we can truly escape the transaction when
	// we build the jobsimDist instance. See luci/gae#23.
	ds := txnBuf.GetNoTxn(j.c)

	logging.Fields{
		"eid": tsk.ExecutionAuth().Id,
	}.Infof(j.c, "jobsim: running new task")

	jtsk, err := j.parsePayload(tsk.Payload().Parameters)
	if err != nil {
		return
	}
	jtsk.ExAuth = *tsk.ExecutionAuth()
	jtsk.Status = jobsimRunnable
	jtsk.StateOrReason = tsk.PreviousResult().Object
	jtsk.CfgName = j.cfg.Name

	key := []*datastore.Key{
		ds.MakeKey(datastore.GetMetaDefault(datastore.GetPLS(jtsk), "kind", ""), 0)}
	if err = ds.AllocateIDs(key); err != nil {
		return
	}

	// transactionally commit the job and a taskqueue task to execute it
	jtsk.ID = fmt.Sprintf("%s|%d", j.jsConfig().Pool, key[0].IntID())
	logging.Infof(j.c, "jobsim: entering transaction")
	err = ds.RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)
		if err := ds.Get(jtsk); err == nil {
			return nil
		}
		logging.Infof(j.c, "jobsim: got jtsk: %s", err)

		if err := ds.Put(jtsk); err != nil {
			return err
		}
		logging.Infof(j.c, "jobsim: put jtsk")

		err := j.cfg.EnqueueTask(c, &taskqueue.Task{
			Payload: []byte(jtsk.ID),
		})
		if err != nil {
			logging.WithError(err).Errorf(j.c, "jobsim: got EnqueueTask error")
			return err
		}
		logging.Infof(j.c, "jobsim: EnqueueTask'd")
		return nil
	}, nil)

	tok = distributor.Token(jtsk.ID)
	return
}

func (j *jobsimDist) Cancel(tok distributor.Token) error {
	jtsk := &jobsimExecution{ID: string(tok)}

	cancelBody := func(ds datastore.Interface) (needWrite bool, err error) {
		if err = ds.Get(jtsk); err != nil {
			return
		}
		if jtsk.Status != jobsimRunnable {
			return
		}
		needWrite = true
		return
	}

	ds := datastore.Get(j.c)
	if needWrite, err := cancelBody(ds); err != nil || !needWrite {
		return err
	}

	return ds.RunInTransaction(func(c context.Context) error {
		ds := datastore.Get(c)
		if needWrite, err := cancelBody(ds); err != nil || !needWrite {
			return err
		}
		jtsk.Status = jobsimCancelled
		return ds.Put(jtsk)
	}, nil)
}

func (j *jobsimDist) GetStatus(tok distributor.Token) (*dm.Result, error) {
	jtsk, err := loadTask(j.c, string(tok))
	if err != nil {
		return nil, err
	}

	return getAttemptResult(jtsk.Status, jtsk.StateOrReason), nil
}

func (j *jobsimDist) InfoURL(tok distributor.Token) string {
	return fmt.Sprintf("jobsim://%s/ver/%s/tok/%s", j.cfg.Name, j.cfg.Version, tok)
}

func (j *jobsimDist) HandleNotification(note *distributor.Notification) (*dm.Result, error) {
	n := &notification{}
	err := json.Unmarshal(note.Data, n)
	if err != nil {
		return nil, err
	}

	return getAttemptResult(n.Status, n.StateOrReason), nil
}

func loadTask(c context.Context, rawTok string) (*jobsimExecution, error) {
	logging.Fields{
		"jobsimId": rawTok,
	}.Infof(c, "jobsim: loading task")

	ds := datastore.Get(c)
	jtsk := &jobsimExecution{ID: string(rawTok)}
	if err := ds.Get(jtsk); err != nil {
		logging.WithError(err).Errorf(c, "jobsim: failed to load task")
		return nil, err
	}

	logging.Fields{
		"eid": jtsk.ExAuth.Id,
	}.Infof(c, "jobsim: execution context")

	return jtsk, nil
}

func (j *jobsimDist) HandleTaskQueueTask(r *http.Request) (notes []*distributor.Notification, err error) {
	// this function is what the lifetime of a remote-execution distributor (like
	// swarming) would do when running a scheduled task.

	// body is a distributor.Token
	rawTok, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	jtsk, err := loadTask(j.c, string(rawTok))
	if err != nil {
		return
	}

	if jtsk.Status != jobsimRunnable {
		logging.Infof(j.c, "jobsim: task is not scheduled")
		return
	}
	jtsk.Status = jobsimRunning
	ds := datastore.Get(j.c)
	if err := ds.Put(jtsk); err != nil {
		logging.WithError(err).Warningf(j.c, "jobsim: failed to update task to Execution_Running")
	}

	state := &state{}
	if err = state.fromPersistentState(jtsk.StateOrReason); err != nil {
		logging.Fields{
			logging.ErrorKey: err,
			"state":          jtsk.StateOrReason,
		}.Errorf(j.c, "jobsim: unable to inflate PersistentState")
		return
	}

	// this is the part that the swarming client/recipe would do.
	err = runJob(j.c, j.cfg.DMHost, state, &jtsk.Calculation, &jtsk.ExAuth, jtsk.CfgName)
	if err != nil {
		jtsk.Status = jobsimFailed
		jtsk.StateOrReason = err.Error()
	} else {
		jtsk.Status = jobsimFinished
		jtsk.StateOrReason = state.toPersistentState()
	}

	err = retry.Retry(j.c, retry.Default, func() error {
		return ds.Put(jtsk)
	}, func(err error, wait time.Duration) {
		logging.Fields{
			logging.ErrorKey: err,
			"wait":           wait,
		}.Warningf(j.c, "jobsim: failed to put task")
	})
	if err != nil {
		logging.WithError(err).Errorf(j.c, "jobsim: FATAL: failed to put task")
		return
	}

	notes = append(notes, &distributor.Notification{
		ID:   jtsk.ExAuth.Id,
		Data: (&notification{jtsk.Status, state.toPersistentState()}).toJSON(),
	})
	return
}

func (j *jobsimDist) Validate(payload string) error {
	_, err := j.parsePayload(payload)
	return err
}

// AddFactory adds this distributor implementation into the distributor
// Registry.
func AddFactory(m distributor.FactoryMap) {
	m[(*jobsim.Config)(nil)] = func(c context.Context, cfg *distributor.Config) (distributor.D, error) {
		return &jobsimDist{c, cfg}, nil
	}
}
