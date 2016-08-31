// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/api/googleapi"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/jsonpb"
	"github.com/luci/gae/service/info"
	swarm "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	googlepb "github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/common/retry"
	sv1 "github.com/luci/luci-go/dm/api/distributor/swarming/v1"
	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/distributor"
	"github.com/luci/luci-go/server/auth"
)

type swarmingDist struct {
	context.Context

	cfg *distributor.Config

	sCfg *sv1.Config
}

var _ distributor.D = (*swarmingDist)(nil)

var swarmBotLookup = map[string]dm.AbnormalFinish_Status{
	"BOT_DIED":  dm.AbnormalFinish_CRASHED,
	"CANCELED":  dm.AbnormalFinish_CANCELLED,
	"EXPIRED":   dm.AbnormalFinish_EXPIRED,
	"TIMED_OUT": dm.AbnormalFinish_TIMED_OUT,
}

func toSwarmMap(m map[string]string) []*swarm.SwarmingRpcsStringPair {
	ret := make([]*swarm.SwarmingRpcsStringPair, 0, len(m))
	for key, value := range m {
		ret = append(ret, &swarm.SwarmingRpcsStringPair{
			Key: key, Value: value})
	}
	return ret
}

func httpClients(c context.Context) (anonC, authC *http.Client) {
	rt, err := auth.GetRPCTransport(c, auth.AsSelf)
	if err != nil {
		// if we can't set up a transport, we're seriously hosed
		panic(err)
	}
	anonTransport, err := auth.GetRPCTransport(c, auth.NoAuth)
	if err != nil {
		panic(err)
	}
	anonC = &http.Client{Transport: anonTransport}
	authC = &http.Client{Transport: rt}
	return
}

func newSwarmClient(c context.Context, cfg *sv1.Config) *swarm.Service {
	_, authC := httpClients(c)
	svc, err := swarm.New(authC)
	if err != nil {
		// can only happen with nil client
		panic(err)
	}
	svc.BasePath = cfg.Swarming.Url + "/_ah/api/swarming/v1/"
	return svc
}

func (d *swarmingDist) Run(tsk *distributor.TaskDescription) (tok distributor.Token, _ time.Duration, err error) {
	auth := tsk.ExecutionAuth()
	id := auth.Id
	desc := tsk.Payload()

	params := &sv1.Parameters{}
	if err = jsonpb.UnmarshalString(desc.DistributorParameters, params); err != nil {
		err = errors.Annotate(err).
			Reason("unmarshalling DistributorParameters").
			InternalReason("These paramaeters were already validated?").
			Err()
		return
	}
	if err = params.Normalize(); err != nil {
		err = errors.Annotate(err).
			Reason("normalizing DistributorParameters").
			InternalReason("These paramaeters were already normalized successfully once?").
			Err()
		return
	}

	isoCtx, _ := context.WithTimeout(d, 30*time.Second)
	iso, err := prepIsolate(isoCtx, d.sCfg.Isolate.Url, tsk, params)
	if err != nil {
		err = errors.Annotate(err).Reason("prepping Isolated").Err()
		return
	}

	topic, token, err := tsk.PrepareTopic()
	if err != nil {
		err = errors.Annotate(err).Reason("preparing topic").Err()
		return
	}

	cipdInput := (*swarm.SwarmingRpcsCipdInput)(nil)
	if len(params.Job.Inputs.Packages) > 0 {
		cipdInput := &swarm.SwarmingRpcsCipdInput{
			Server: params.Job.Inputs.CipdServer,
		}
		for _, pkg := range params.Job.Inputs.Packages {
			cipdInput.Packages = append(cipdInput.Packages, &swarm.SwarmingRpcsCipdPackage{
				PackageName: pkg.Name,
				Path:        pkg.Path,
				Version:     pkg.Version,
			})
		}
	}

	dims := []*swarm.SwarmingRpcsStringPair(nil)
	for key, value := range params.Scheduling.Dimensions {
		dims = append(dims, &swarm.SwarmingRpcsStringPair{Key: key, Value: value})
	}

	prefix := params.Meta.NamePrefix
	if len(prefix) > 0 {
		prefix += " / "
	}

	tags := []string{
		"requestor:DM",
		"requestor:" + info.Get(d).TrimmedAppID(),
		"requestor:swarming_v1",
		fmt.Sprintf("quest:%s", id.Quest),
		fmt.Sprintf("attempt:%s|%d", id.Quest, id.Attempt),
		fmt.Sprintf("execution:%s|%d|%d", id.Quest, id.Attempt, id.Id),
	}

	rslt := (*swarm.SwarmingRpcsTaskRequestMetadata)(nil)
	err = retry.Retry(d, retry.Default, func() (err error) {
		rpcCtx, _ := context.WithTimeout(d, 10*time.Second)
		rslt, err = newSwarmClient(rpcCtx, d.sCfg).Tasks.New(&swarm.SwarmingRpcsNewTaskRequest{
			ExpirationSecs: int64(desc.Meta.Timeouts.Start.Duration().Seconds()),
			Name:           fmt.Sprintf("%s%s|%d|%d", prefix, id.Quest, id.Attempt, id.Id),

			// Priority is already pre-Normalize()'d
			Priority: int64(params.Scheduling.Priority),

			Properties: &swarm.SwarmingRpcsTaskProperties{
				CipdInput:            cipdInput,
				Dimensions:           toSwarmMap(params.Scheduling.Dimensions),
				Env:                  toSwarmMap(params.Job.Env),
				ExecutionTimeoutSecs: int64(desc.Meta.Timeouts.Run.Duration().Seconds()),
				GracePeriodSecs:      int64(desc.Meta.Timeouts.Stop.Duration().Seconds()),
				IoTimeoutSecs:        int64(params.Scheduling.IoTimeout.Duration().Seconds()),
				InputsRef:            iso,
			},

			PubsubTopic:     topic.String(),
			PubsubAuthToken: token,

			Tags: tags,
		}).Context(rpcCtx).Do()
		return
	}, retry.LogCallback(d, "swarm.Tasks.New"))
	if err != nil {
		err = errors.Annotate(err).Reason("calling swarm.Tasks.New").Err()
		return
	}

	tok = distributor.Token(rslt.TaskId)
	return
}

func (d *swarmingDist) Cancel(tok distributor.Token) error {
	return retry.Retry(d, retry.Default, func() (err error) {
		ctx, _ := context.WithTimeout(d, 10*time.Second)
		_, err = newSwarmClient(ctx, d.sCfg).Task.Cancel(string(tok)).Context(ctx).Do()
		return
	}, retry.LogCallback(d, "swarm.Task.Cancel"))
}

func (d *swarmingDist) GetStatus(tok distributor.Token) (*dm.Result, error) {
	rslt := (*swarm.SwarmingRpcsTaskResult)(nil)

	err := retry.Retry(d, retry.Default, func() (err error) {
		ctx, _ := context.WithTimeout(d, 10*time.Second)
		rslt, err = newSwarmClient(ctx, d.sCfg).Task.Result(string(tok)).Context(ctx).Do()
		return
	}, retry.LogCallback(d, fmt.Sprintf("swarm.Task.Result(%s)", tok)))
	if err != nil {
		if gerr := err.(*googleapi.Error); gerr != nil {
			if gerr.Code == http.StatusNotFound {
				return &dm.Result{
					AbnormalFinish: &dm.AbnormalFinish{
						Status: dm.AbnormalFinish_MISSING,
						Reason: "swarming: notFound",
					},
				}, nil
			}
		}

		return nil, err
	}
	ret := &dm.Result{}

	switch rslt.State {
	case "PENDING", "RUNNING":
		return nil, nil

	case "COMPLETED":
		if rslt.InternalFailure {
			ret.AbnormalFinish = &dm.AbnormalFinish{
				Status: dm.AbnormalFinish_CRASHED,
				Reason: fmt.Sprintf("swarming: COMPLETED/InternalFailure(%d)", rslt.ExitCode),
			}
			break
		}

		retData := &sv1.Result{
			ExitCode: rslt.ExitCode,
		}
		if ref := rslt.OutputsRef; ref != nil {
			retData.IsolatedOutdir = &sv1.IsolatedRef{
				Id: ref.Isolated, Server: ref.Isolatedserver}
		}
		data, err := (&jsonpb.Marshaler{OrigName: true}).MarshalToString(retData)
		if err != nil {
			panic(err)
		}

		ret.Data = &dm.JsonResult{
			Object: data,
			Size:   uint32(len(data)),
			Expiration: googlepb.NewTimestamp(
				clock.Now(d).Add(d.sCfg.Isolate.Expiration.Duration())),
		}

	default:
		if bad, ok := swarmBotLookup[rslt.State]; ok {
			ret.AbnormalFinish = &dm.AbnormalFinish{
				Status: bad,
				Reason: fmt.Sprintf("swarming: %s", rslt.State),
			}
		} else {
			ret.AbnormalFinish = &dm.AbnormalFinish{
				Status: dm.AbnormalFinish_RESULT_MALFORMED,
				Reason: fmt.Sprintf("swarming: unknown state %s", rslt.State),
			}
		}
	}

	return ret, nil
}

func (d *swarmingDist) InfoURL(tok distributor.Token) string {
	return fmt.Sprintf("%s/user/task/%s", d.sCfg.Swarming.Url, tok)
}

func (d *swarmingDist) HandleNotification(notification *distributor.Notification) (*dm.Result, error) {
	type Data struct {
		TaskID distributor.Token `json:"task_id"`
	}
	dat := &Data{}
	if err := json.Unmarshal(notification.Data, dat); err != nil {
		logging.Fields{"payload": notification.Data}.Errorf(
			d, "Could not unmarshal swarming payload! relying on timeout.")
		return nil, nil
	}
	return d.GetStatus(dat.TaskID)
}

func (*swarmingDist) HandleTaskQueueTask(r *http.Request) ([]*distributor.Notification, error) {
	return nil, nil
}

func (*swarmingDist) Validate(payload string) error {
	msg := &sv1.Parameters{}
	if err := jsonpb.UnmarshalString(payload, msg); err != nil {
		return errors.Annotate(err).Reason("unmarshal").D("payload", payload).Err()
	}
	return errors.Annotate(msg.Normalize()).Reason("normalize").D("payload", payload).Err()
}

func factory(c context.Context, cfg *distributor.Config) (distributor.D, error) {
	return &swarmingDist{c, cfg, cfg.Content.(*sv1.Config)}, nil
}

// AddFactory adds this distributor implementation into the distributor
// Registry.
func AddFactory(m distributor.FactoryMap) {
	m[(*sv1.Config)(nil)] = factory
}
