// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/api/isolate/isolateservice/v1"
	swarm "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/isolated"
	"github.com/luci/luci-go/common/isolatedclient"
	"github.com/luci/luci-go/common/sync/parallel"
	sv1 "github.com/luci/luci-go/dm/api/distributor/swarming/v1"
	"github.com/luci/luci-go/dm/appengine/distributor"
	"golang.org/x/net/context"
)

const prevPath = ".dm/previous_execution.json"
const exAuthPath = ".dm/execution_auth.json"
const descPath = ".dm/quest_description.json"

func mkFile(data []byte) *isolated.File {
	mode := 0444
	size := int64(len(data))
	return &isolated.File{
		Digest: isolated.HashBytes(data),
		Mode:   &mode,
		Size:   &size,
	}
}

func mkMsgFile(pb proto.Message) ([]byte, *isolated.File) {
	buf := &bytes.Buffer{}
	if err := (&jsonpb.Marshaler{OrigName: true}).Marshal(buf, pb); err != nil {
		panic(err)
	}
	data := buf.Bytes()
	return data, mkFile(data)
}

func mkIsolated(c context.Context, params *sv1.Parameters, prevFile, descFile, authFile *isolated.File) ([]byte, *isolated.File) {
	cmdReplacer := strings.NewReplacer(
		"${DM.PREVIOUS.EXECUTION.STATE:PATH}", prevPath,
		"${DM.QUEST.DATA.DESC:PATH}", descPath,
		"${DM.EXECUTION.AUTH:PATH}", exAuthPath,
		"${DM.HOST}", info.Get(c).DefaultVersionHostname(),
	)

	iso := isolated.New()
	iso.Command = make([]string, len(params.Job.Command))
	for i, tok := range params.Job.Command {
		iso.Command[i] = cmdReplacer.Replace(tok)
	}
	iso.Includes = make(isolated.HexDigests, len(params.Job.Inputs.Isolated))
	for i, input := range params.Job.Inputs.Isolated {
		iso.Includes[i] = isolated.HexDigest(input.Id)
	}

	iso.Files[prevPath] = *prevFile
	iso.Files[exAuthPath] = *authFile
	iso.Files[descPath] = *descFile

	isoData, err := json.Marshal(iso)
	if err != nil {
		panic(err)
	}
	isoFile := mkFile(isoData)

	return isoData, isoFile
}

type isoChunk struct {
	data  []byte
	isIso bool
	file  *isolated.File
}

func pushIsolate(c context.Context, isolateURL string, chunks []isoChunk) error {
	dgsts := make([]*isolateservice.HandlersEndpointsV1Digest, len(chunks))
	for i, chnk := range chunks {
		dgsts[i] = &isolateservice.HandlersEndpointsV1Digest{
			Digest: string(chnk.file.Digest), Size: *chnk.file.Size,
			IsIsolated: chnk.isIso}
	}

	anonC, authC := httpClients(c)

	isoClient := isolatedclient.New(
		anonC, authC, isolateURL, isolatedclient.DefaultNamespace, nil)
	states, err := isoClient.Contains(c, dgsts)
	if err != nil {
		err = errors.Annotate(err).
			D("count", len(dgsts)).
			Reason("checking containment for %(count)d digests").
			Err()
		return err
	}
	return parallel.FanOutIn(func(ch chan<- func() error) {
		for i, st := range states {
			if st != nil {
				i, st := i, st
				ch <- func() error {
					return isoClient.Push(c, st, isolatedclient.NewBytesSource(chunks[i].data))
				}
			}
		}
	})
}

func prepIsolate(c context.Context, isolateURL string, tsk *distributor.TaskDescription, params *sv1.Parameters) (*swarm.SwarmingRpcsFilesRef, error) {
	prevData := []byte("{}")
	if tsk.PreviousResult() != nil {
		prevData = []byte(tsk.PreviousResult().Object)
	}
	prevFile := mkFile(prevData)
	authData, authFile := mkMsgFile(tsk.ExecutionAuth())
	descData, descFile := mkMsgFile(tsk.Payload())
	isoData, isoFile := mkIsolated(c, params, prevFile, descFile, authFile)

	err := pushIsolate(c, isolateURL, []isoChunk{
		{data: prevData, file: prevFile},
		{data: authData, file: authFile},
		{data: descData, file: descFile},
		{data: isoData, file: isoFile, isIso: true},
	})
	if err != nil {
		err = errors.Annotate(err).Reason("pushing new Isolated").Err()
		return nil, err
	}

	return &swarm.SwarmingRpcsFilesRef{
		Isolated:       string(isoFile.Digest),
		Isolatedserver: isolateURL,
		Namespace:      isolatedclient.DefaultNamespace,
	}, nil
}
