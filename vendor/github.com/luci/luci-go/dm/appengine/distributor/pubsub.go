// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/tokens"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

const notifyTopicSuffix = "dm-distributor-notify"

// PubsubReceiver is the HTTP handler that processes incoming pubsub events
// delivered to topics prepared with TaskDescription.PrepareTopic, and routes
// them to the appropriate distributor implementation's HandleNotification
// method.
//
// It requires that a Registry be installed in c via WithRegistry.
func PubsubReceiver(ctx *router.Context) {
	c, rw, r := ctx.Context, ctx.Writer, ctx.Request
	defer r.Body.Close()

	type PubsubMessage struct {
		Attributes map[string]string `json:"attributes"`
		Data       []byte            `json:"data"`
		MessageID  string            `json:"message_id"`
	}
	type PubsubPushMessage struct {
		Message      PubsubMessage `json:"message"`
		Subscription string        `json:"subscription"`
	}
	psm := &PubsubPushMessage{}

	if err := json.NewDecoder(r.Body).Decode(psm); err != nil {
		logging.WithError(err).Errorf(c, "Failed to parse pubsub message")
		http.Error(rw, "Failed to parse pubsub message", http.StatusInternalServerError)
		return
	}

	eid, cfgName, err := decodeAuthToken(c, psm.Message.Attributes["auth_token"])
	if err != nil {
		logging.WithError(err).Errorf(c, "bad auth_token")
		// Acknowledge this message, since it'll never be valid.
		rw.WriteHeader(http.StatusNoContent)
		return
	}

	// remove "auth_token" from Attributes to avoid having it pass to the
	// distributor.
	delete(psm.Message.Attributes, "auth_token")

	err = tumble.RunMutation(c, &NotifyExecution{
		cfgName, &Notification{eid, psm.Message.Data, psm.Message.Attributes},
	})
	if err != nil {
		// TODO(riannucci): distinguish between transient/non-transient failures.
		logging.WithError(err).Errorf(c, "failed to NotifyExecution")
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

// pubsubAuthToken describes how to generate HMAC protected tokens used to
// authenticate PubSub messages.
var pubsubAuthToken = tokens.TokenKind{
	Algo:       tokens.TokenAlgoHmacSHA256,
	Expiration: 48 * time.Hour,
	SecretKey:  "pubsub_auth_token",
	Version:    1,
}

func encodeAuthToken(c context.Context, eid *dm.Execution_ID, cfgName string) (string, error) {
	return pubsubAuthToken.Generate(c, nil, map[string]string{
		"quest":     eid.Quest,
		"attempt":   strconv.FormatUint(uint64(eid.Attempt), 10),
		"execution": strconv.FormatUint(uint64(eid.Id), 10),
		"cfgName":   cfgName,
	}, 0)
}

func decodeAuthToken(c context.Context, authToken string) (eid *dm.Execution_ID, cfgName string, err error) {
	items, err := pubsubAuthToken.Validate(c, authToken, nil)
	if err != nil {
		return
	}
	quest, qok := items["quest"]
	attempt, aok := items["attempt"]
	execution, eok := items["execution"]
	if !qok || !aok || !eok {
		err = fmt.Errorf("missing keys: %v", items)
		return
	}
	attemptNum, err := strconv.ParseUint(attempt, 10, 32)
	if err != nil {
		return
	}
	executionNum, err := strconv.ParseUint(execution, 10, 32)
	if err != nil {
		return
	}
	eid = dm.NewExecutionID(quest, uint32(attemptNum), uint32(executionNum))

	cfgName, ok := items["cfgName"]
	if !ok {
		err = fmt.Errorf("missing config name")
	}

	return
}
