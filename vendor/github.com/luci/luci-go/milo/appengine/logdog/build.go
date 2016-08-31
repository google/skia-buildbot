// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logdog

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/proto/google"
	miloProto "github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/logdog/internal"
	"github.com/luci/luci-go/milo/common/miloerror"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/memcache"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

const (
	// intermediateCacheLifetime is the amount of time to cache intermediate (non-
	// terminal) annotation streams. Terminal annotation streams are cached
	// indefinitely.
	intermediateCacheLifetime = 10 * time.Second

	// defaultLogDogHost is the default LogDog host, if one isn't specified via
	// query string.
	defaultLogDogHost = "luci-logdog.appspot.com"
)

type annotationStreamRequest struct {
	*AnnotationStream

	// host is the name of the LogDog host.
	host string

	project config.ProjectName
	path    types.StreamPath

	// logDogClient is the HTTP client to use for LogDog communication.
	logDogClient http.Client

	// cs is the unmarshalled annotation stream Step and associated data.
	cs internal.CachedStep
}

func (as *annotationStreamRequest) normalize() error {
	if err := as.project.Validate(); err != nil {
		return &miloerror.Error{
			Message: "Invalid project name",
			Code:    http.StatusBadRequest,
		}
	}

	if err := as.path.Validate(); err != nil {
		return &miloerror.Error{
			Message: fmt.Sprintf("Invalid log stream path %q: %s", as.path, err),
			Code:    http.StatusBadRequest,
		}
	}

	// Get the host. We normalize it to lowercase and trim spaces since we use
	// it as a memcache key.
	as.host = strings.ToLower(strings.TrimSpace(as.host))
	if as.host == "" {
		as.host = defaultLogDogHost
	}
	if strings.ContainsRune(as.host, '/') {
		return errors.New("invalid host name")
	}

	return nil
}

func (as *annotationStreamRequest) memcacheKey() string {
	return fmt.Sprintf("logdog/%s/%s/%s", as.host, as.project, as.path)
}

func (as *annotationStreamRequest) load(c context.Context) error {
	// Load from memcache, if possible. If an error occurs, we will proceed as if
	// no CachedStep was available.
	mcKey := as.memcacheKey()
	mcItem, err := memcache.Get(c).Get(mcKey)
	switch err {
	case nil:
		if err := proto.Unmarshal(mcItem.Value(), &as.cs); err == nil {
			return nil
		}

		// We could not unmarshal the cached value. Try and delete it from
		// memcache, since it's invalid.
		log.Fields{
			log.ErrorKey:  err,
			"memcacheKey": mcKey,
		}.Warningf(c, "Failed to unmarshal cached annotation protobuf.")
		if err := memcache.Get(c).Delete(mcKey); err != nil {
			log.WithError(err).Warningf(c, "Failed to delete invalid annotation protobuf memcache entry.")
		}

	case memcache.ErrCacheMiss:
		break

	default:
		log.Fields{
			log.ErrorKey:  err,
			"memcacheKey": mcKey,
		}.Errorf(c, "Failed to load annotation protobuf memcache cached step.")
	}

	// Load from LogDog directly.
	client := logdog.NewLogsPRPCClient(&prpc.Client{
		C:    &as.logDogClient,
		Host: as.host,
	})

	log.Fields{
		"project": as.project,
		"path":    as.path,
		"host":    as.host,
	}.Infof(c, "Making tail request to LogDog to fetch annotation stream.")
	resp, err := client.Tail(c, &logdog.TailRequest{
		Project: string(as.project),
		Path:    string(as.path),
		State:   true,
	})
	switch code := grpcutil.Code(err); code {
	case codes.OK:
		break

	case codes.NotFound:
		return &miloerror.Error{
			Message: "Stream not found",
			Code:    http.StatusNotFound,
		}

	default:
		// TODO: Once we switch to delegation tokens and are making the request on
		// behalf of a user rather than the Milo service, handle PermissionDenied.
		log.Fields{
			log.ErrorKey: err,
			"code":       code,
		}.Errorf(c, "Failed to load LogDog annotation stream.")
		return &miloerror.Error{
			Message: "Failed to load stream",
			Code:    http.StatusInternalServerError,
		}
	}

	// Make sure that this is an annotation stream.
	switch {
	case resp.Desc.ContentType != miloProto.ContentTypeAnnotations:
		return &miloerror.Error{
			Message: "Requested stream is not a Milo annotation protobuf",
			Code:    http.StatusBadRequest,
		}

	case resp.Desc.StreamType != logpb.StreamType_DATAGRAM:
		return &miloerror.Error{
			Message: "Requested stream is not a datagram stream",
			Code:    http.StatusBadRequest,
		}

	case len(resp.Logs) == 0:
		// No annotation stream data, so render a minimal page.
		return nil
	}

	// Get the last log entry in the stream. In reality, this will be index 0,
	// since the "Tail" call should only return one log entry.
	latestStream := resp.Logs[len(resp.Logs)-1]
	dg := latestStream.GetDatagram()
	switch {
	case dg == nil:
		return &miloerror.Error{
			Message: "Datagram stream does not have datagram data",
			Code:    http.StatusInternalServerError,
		}

	case dg.Partial != nil && !(dg.Partial.Index == 0 && dg.Partial.Last):
		// LogDog splits large datagrams into consecutive fragments. If the
		// annotation state is fragmented, a reconstruction algorithm will have to
		// be employed here to build the full datagram before processing.
		//
		// At the moment, no annotation streams are expected to be anywhere close to
		// this large, so we're going to handle this case by erroring. A
		// reconstruction algorithm would look like:
		// 1) "Tail" to get the latest datagram, identify it as partial.
		// 1a) Perform a bounds check on the total datagram size to ensure that it
		//     can be safely reconstructed.
		// 2) Determine if it's the last partial index. If not, then the latest
		//     datagram is incomplete. Determine our initial datagram's stream index
		//     the by subtracting the partial index from this message's stream index.
		// 2a) If this datagram index is "0", the first datagram in the stream is
		//     partial and all of the data isn't here, so treat this as "no data".
		// 2b) Otherwise, goto (1), using "Get" request on the datagram index minus
		//     one to get the previous datagram.
		// 3) Issue a "Get" request for our initial datagram index through the index
		//     preceding ours.
		// 4) Reassemble the binary data from the full set of datagrams.
		return &miloerror.Error{
			Message: "Partial datagram streams are not supported yet",
			Code:    http.StatusNotImplemented,
		}
	}

	// Attempt to decode the Step protobuf.
	var step miloProto.Step
	if err := proto.Unmarshal(dg.Data, &step); err != nil {
		return &miloerror.Error{
			Message: "Failed to unmarshal annotation protobuf",
			Code:    http.StatusInternalServerError,
		}
	}

	var latestEndedTime time.Time
	for _, sub := range step.Substep {
		switch t := sub.Substep.(type) {
		case *miloProto.Step_Substep_AnnotationStream:
			// TODO(hinoka,dnj): Implement recursive / embedded substream fetching if
			// specified.
			log.Warningf(c, "Annotation stream links LogDog substream [%+v], not supported!", t.AnnotationStream)

		case *miloProto.Step_Substep_Step:
			endedTime := t.Step.Ended.Time()
			if t.Step.Ended != nil && endedTime.After(latestEndedTime) {
				latestEndedTime = endedTime
			}
		}
	}
	if latestEndedTime.IsZero() {
		// No substep had an ended time :(
		latestEndedTime = step.Started.Time()
	}

	// Build our CachedStep.
	as.cs = internal.CachedStep{
		Step:     &step,
		Finished: (resp.State.TerminalIndex >= 0 && latestStream.StreamIndex == uint64(resp.State.TerminalIndex)),
	}

	// Annotee is apparently not putting an ended time on some annotation protos.
	// This hack will ensure that a finished build will always have an ended time.
	if as.cs.Finished && as.cs.Step.Ended == nil {
		as.cs.Step.Ended = google.NewTimestamp(latestEndedTime)
	}

	// Marshal and cache the step. If this is the final protobuf in the stream,
	// cache it indefinitely; otherwise, cache it for intermediateCacheLifetime.
	//
	// If this fails, it is non-fatal.
	mcData, err := proto.Marshal(&as.cs)
	if err == nil {
		mcItem = memcache.Get(c).NewItem(mcKey)
		if !as.cs.Finished {
			mcItem.SetExpiration(intermediateCacheLifetime)
		}
		mcItem.SetValue(mcData)
		if err := memcache.Get(c).Set(mcItem); err != nil {
			log.WithError(err).Warningf(c, "Failed to cache annotation protobuf CachedStep.")
		}
	} else {
		log.WithError(err).Warningf(c, "Failed to marshal annotation protobuf CachedStep.")
	}

	return nil
}

func (as *annotationStreamRequest) toMiloBuild(c context.Context) *resp.MiloBuild {
	prefix, name := as.path.Split()

	// Prepare a Streams object with only one stream.
	streams := Streams{
		MainStream: &Stream{
			Server:     as.host,
			Prefix:     string(prefix),
			Path:       string(name),
			IsDatagram: true,
			Data:       as.cs.Step,
			Closed:     as.cs.Finished,
		},
	}

	var (
		build resp.MiloBuild
		ub    = logDogURLBuilder{
			project: as.project,
			host:    as.host,
			prefix:  prefix,
		}
	)
	AddLogDogToBuild(c, &ub, &streams, &build)
	return &build
}

type logDogURLBuilder struct {
	host    string
	prefix  types.StreamName
	project config.ProjectName
}

func (b *logDogURLBuilder) BuildLink(l *miloProto.Link) *resp.Link {
	switch t := l.Value.(type) {
	case *miloProto.Link_LogdogStream:
		ls := t.LogdogStream

		server := ls.Server
		if server == "" {
			server = b.host
		}

		prefix := types.StreamName(ls.Prefix)
		if prefix == "" {
			prefix = b.prefix
		}

		path := fmt.Sprintf("%s/%s", b.project, prefix.Join(types.StreamName(ls.Name)))
		u := url.URL{
			Scheme: "https",
			Host:   server,
			Path:   "v/",
			RawQuery: url.Values{
				"s": []string{string(path)},
			}.Encode(),
		}

		link := resp.Link{
			Label: l.Label,
			URL:   u.String(),
		}
		if link.Label == "" {
			link.Label = ls.Name
		}
		return &link

	case *miloProto.Link_Url:
		link := resp.Link{
			Label: l.Label,
			URL:   t.Url,
		}
		if link.Label == "" {
			link.Label = "unnamed"
		}
		return &link

	default:
		// Don't know how to render.
		return nil
	}
}
