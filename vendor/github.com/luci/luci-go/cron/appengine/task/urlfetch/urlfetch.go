// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package urlfetch implements cron tasks that just make HTTP call.
package urlfetch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/api/pubsub/v1"

	"github.com/luci/gae/service/urlfetch"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/cron/appengine/messages"
	"github.com/luci/luci-go/cron/appengine/task"
)

// TaskManager implements task.Manager interface for tasks defined with
// UrlFetchTask proto message
type TaskManager struct {
}

// Name is part of Manager interface.
func (m TaskManager) Name() string {
	return "url_fetch"
}

// ProtoMessageType is part of Manager interface.
func (m TaskManager) ProtoMessageType() proto.Message {
	return (*messages.UrlFetchTask)(nil)
}

// ValidateProtoMessage is part of Manager interface.
func (m TaskManager) ValidateProtoMessage(msg proto.Message) error {
	cfg, ok := msg.(*messages.UrlFetchTask)
	if !ok {
		return fmt.Errorf("wrong type %T, expecting *messages.UrlFetchTask", msg)
	}

	// Validate 'method' field.
	// TODO(vadimsh): Add more methods (POST, PUT) when 'Body' is added.
	goodMethods := map[string]bool{"GET": true}
	if !goodMethods[cfg.GetMethod()] {
		return fmt.Errorf("unsupported HTTP method: %q", cfg.GetMethod())
	}

	// Validate 'url' field.
	if cfg.GetUrl() == "" {
		return fmt.Errorf("field 'url' is required")
	}
	u, err := url.Parse(cfg.GetUrl())
	if err != nil {
		return fmt.Errorf("invalid URL %q: %s", cfg.GetUrl(), err)
	}
	if !u.IsAbs() {
		return fmt.Errorf("not an absolute url: %q", cfg.GetUrl())
	}

	// Validate 'timeout_sec' field. GAE task queue request deadline is 10 min, so
	// limit URL fetch call duration to 8 min (giving 2 min to spare).
	if cfg.GetTimeoutSec() < 1 {
		return fmt.Errorf("minimum allowed 'timeout_sec' is 1 sec, got %d", cfg.GetTimeoutSec())
	}
	if cfg.GetTimeoutSec() > 480 {
		return fmt.Errorf("maximum allowed 'timeout_sec' is 480 sec, got %d", cfg.GetTimeoutSec())
	}

	return nil
}

// LaunchTask is part of Manager interface.
func (m TaskManager) LaunchTask(c context.Context, ctl task.Controller) error {
	cfg := ctl.Task().(*messages.UrlFetchTask)
	started := clock.Now(c)
	ctl.DebugLog("%s %s", cfg.GetMethod(), cfg.GetUrl())

	// There must be no errors here in reality, since cfg is validated already by
	// ValidateProtoMessage.
	u, err := url.Parse(cfg.GetUrl())
	if err != nil {
		return err
	}

	type tuple struct {
		resp *http.Response
		body []byte // first 4Kb of the response, for debug log
		err  error
	}
	result := make(chan tuple)

	// Do the fetch asynchronously with datastore update.
	go func() {
		defer close(result)
		c, _ = clock.WithTimeout(c, time.Duration(cfg.GetTimeoutSec())*time.Second)
		client := &http.Client{Transport: urlfetch.Get(c)}
		resp, err := client.Do(&http.Request{
			Method: cfg.GetMethod(),
			URL:    u,
		})
		if err != nil {
			result <- tuple{nil, nil, err}
			return
		}
		defer resp.Body.Close()
		// Ignore read errors here. HTTP status code is set, it's the main output
		// of the operation. Read 4K only since we use body only for debug message
		// that is limited in size.
		buf := bytes.Buffer{}
		io.CopyN(&buf, resp.Body, 4096)
		result <- tuple{resp, buf.Bytes(), nil}
	}()

	// Notify outside world that the task is running (since URL fetch can take up
	// to 8 minutes). Ignore errors. As long as final Save is OK, we don't care
	// about this one.
	ctl.State().Status = task.StatusRunning
	if err := ctl.Save(); err != nil {
		logging.Warningf(c, "Failed to save invocation state: %s", err)
	}

	// Wait for completion.
	res := <-result

	duration := clock.Now(c).Sub(started)
	status := task.StatusSucceeded
	if res.err != nil || res.resp.StatusCode >= 400 {
		status = task.StatusFailed
	}

	ctl.DebugLog("Finished with overall status %s in %s", status, duration)
	if res.err != nil {
		ctl.DebugLog("URL fetch error: %s", res.err)
	} else {
		ctl.DebugLog(dumpResponse(res.resp, res.body))
	}
	ctl.State().Status = status
	return nil
}

// AbortTask is part of Manager interface.
func (m TaskManager) AbortTask(c context.Context, ctl task.Controller) error {
	return nil
}

// HandleNotification is part of Manager interface.
func (m TaskManager) HandleNotification(c context.Context, ctl task.Controller, msg *pubsub.PubsubMessage) error {
	return errors.New("not implemented")
}

////////////////////////////////////////////////////////////////////////////////

// dumpResponse converts http.Response to text for the invocation debug log.
func dumpResponse(resp *http.Response, body []byte) string {
	out := &bytes.Buffer{}
	fmt.Fprintln(out, resp.Status)
	resp.Header.Write(out)
	fmt.Fprintln(out)
	if len(body) == 0 {
		fmt.Fprintln(out, "<empty body>")
	} else if isTextContent(resp.Header) {
		out.Write(body)
		if body[len(body)-1] != '\n' {
			fmt.Fprintln(out)
		}
		if int64(len(body)) < resp.ContentLength {
			fmt.Fprintln(out, "<truncated>")
		}
	} else {
		fmt.Fprintln(out, "<binary response>")
	}
	return out.String()
}

var textContentTypes = []string{
	"text/",
	"application/json",
	"application/xml",
}

// isTextContent returns True if Content-Type header corresponds to some
// readable text type.
func isTextContent(h http.Header) bool {
	for _, header := range h["Content-Type"] {
		for _, good := range textContentTypes {
			if strings.HasPrefix(header, good) {
				return true
			}
		}
	}
	return false
}
