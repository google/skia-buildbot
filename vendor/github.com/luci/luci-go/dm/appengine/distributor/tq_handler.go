// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/tumble"
)

const handlerPattern = "/tq/distributor/:cfgName"

func handlerPath(cfgName string) string {
	return strings.Replace(handlerPattern, ":cfgName", url.QueryEscape(cfgName), 1)
}

// TaskQueueHandler is the http handler that routes taskqueue tasks made with
// Config.EnqueueTask to a distributor's HandleTaskQueueTask method.
//
// This requires that ctx.Context already have a Registry installed via the
// WithRegistry method.
func TaskQueueHandler(ctx *router.Context) {
	c, rw, r, p := ctx.Context, ctx.Writer, ctx.Request, ctx.Params
	defer r.Body.Close()

	cfgName := p.ByName("cfgName")
	dist, _, err := GetRegistry(c).MakeDistributor(c, cfgName)
	if err != nil {
		logging.Fields{"error": err, "cfg": cfgName}.Errorf(c, "Failed to make distributor")
		http.Error(rw, "bad distributor", http.StatusBadRequest)
		return
	}
	notifications, err := dist.HandleTaskQueueTask(r)
	if err != nil {
		logging.Fields{"error": err, "cfg": cfgName}.Errorf(c, "Failed to handle taskqueue task")
		http.Error(rw, "failure to execute handler", http.StatusInternalServerError)
		return
	}
	if len(notifications) > 0 {
		muts := make([]tumble.Mutation, 0, len(notifications))
		for _, notify := range notifications {
			if notify != nil {
				muts = append(muts, &NotifyExecution{cfgName, notify})
			}
		}
		err = tumble.AddToJournal(c, muts...)
		if err != nil {
			logging.Fields{"error": err, "cfg": cfgName}.Errorf(c, "Failed to handle notifications")
			http.Error(rw, "failure to handle notifications", http.StatusInternalServerError)
			return
		}
	}
	rw.WriteHeader(http.StatusOK)
}
