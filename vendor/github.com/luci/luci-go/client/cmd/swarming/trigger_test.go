// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"testing"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/luci/luci-go/common/flag/stringmapflag"
	"github.com/maruel/ut"
)

// Make sure that stringmapflag.Value are returned as sorted arrays.
func TestMapToArray(t *testing.T) {
	type item struct {
		m stringmapflag.Value
		a []*swarming.SwarmingRpcsStringPair
	}

	data := []item{
		{
			m: stringmapflag.Value{},
			a: []*swarming.SwarmingRpcsStringPair{},
		},
		{
			m: stringmapflag.Value{
				"foo": "bar",
			},
			a: []*swarming.SwarmingRpcsStringPair{
				{Key: "foo", Value: "bar"},
			},
		},
		{
			m: stringmapflag.Value{
				"foo":  "bar",
				"toto": "fifi",
			},
			a: []*swarming.SwarmingRpcsStringPair{
				{Key: "foo", Value: "bar"},
				{Key: "toto", Value: "fifi"},
			},
		},
		{
			m: stringmapflag.Value{
				"toto": "fifi",
				"foo":  "bar",
			},
			a: []*swarming.SwarmingRpcsStringPair{
				{Key: "foo", Value: "bar"},
				{Key: "toto", Value: "fifi"},
			},
		},
	}

	for _, item := range data {
		a := mapToArray(item.m)
		ut.ExpectEqual(t, len(item.m), len(a))
		ut.ExpectEqual(t, item.a, a)
	}
}

func TestNamePartFromDimensions(t *testing.T) {
	type item struct {
		m    stringmapflag.Value
		part string
	}

	data := []item{
		{
			m:    stringmapflag.Value{},
			part: "",
		},
		{
			m: stringmapflag.Value{
				"foo": "bar",
			},
			part: "foo=bar",
		},
		{
			m: stringmapflag.Value{
				"foo":  "bar",
				"toto": "fifi",
			},
			part: "foo=bar_toto=fifi",
		},
		{
			m: stringmapflag.Value{
				"toto": "fifi",
				"foo":  "bar",
			},
			part: "foo=bar_toto=fifi",
		},
	}

	for _, item := range data {
		part := namePartFromDimensions(item.m)
		ut.ExpectEqual(t, item.part, part)
	}
}

func TestParse_NoArgs(t *testing.T) {
	c := triggerRun{}
	c.Init()

	err := c.Parse([]string{})
	ut.AssertEqual(t, errors.New("must provide -server"), err)
}

func TestParse_NoDimension(t *testing.T) {
	c := triggerRun{}
	c.Init()

	err := c.GetFlags().Parse([]string{"-server", "http://localhost:9050"})

	err = c.Parse([]string{})
	ut.AssertEqual(t, errors.New("please at least specify one dimension"), err)
}

func TestParse_NoIsolated(t *testing.T) {
	c := triggerRun{}
	c.Init()

	err := c.GetFlags().Parse([]string{
		"-server", "http://localhost:9050",
		"-dimension", "os=Ubuntu",
	})

	err = c.Parse([]string{})
	ut.AssertEqual(t, errors.New("please use -isolated to specify hash"), err)
}

func TestParse_BadIsolated(t *testing.T) {
	c := triggerRun{}
	c.Init()

	err := c.GetFlags().Parse([]string{
		"-server", "http://localhost:9050",
		"-dimension", "os=Ubuntu",
		"-isolated", "0123456789",
	})

	err = c.Parse([]string{})
	ut.AssertEqual(t, errors.New("invalid hash"), err)
}

func TestParse_RawNoArgs(t *testing.T) {
	c := triggerRun{}
	c.Init()

	err := c.GetFlags().Parse([]string{
		"-server", "http://localhost:9050",
		"-dimension", "os=Ubuntu",
		"-isolated", "0123456789012345678901234567890123456789",
		"-raw-cmd",
	})

	err = c.Parse([]string{})
	ut.AssertEqual(t, errors.New("arguments with -raw-cmd should be passed after -- as command delimiter"), err)
}

func TestParse_RawAndIsolateServer(t *testing.T) {
	c := triggerRun{}
	c.Init()

	err := c.GetFlags().Parse([]string{
		"-server", "http://localhost:9050",
		"-dimension", "os=Ubuntu",
		"-isolated", "0123456789012345678901234567890123456789",
		"-raw-cmd",
		"-isolate-server", "http://localhost:10050",
	})

	err = c.Parse([]string{"args1"})
	ut.AssertEqual(t, errors.New("can't use both -raw-cmd and -isolate-server"), err)
}

func TestProcessTriggerOptions_WithRawArgs(t *testing.T) {
	c := triggerRun{}
	c.Init()
	c.commonFlags.serverURL = "http://localhost:9050"
	c.isolateServer = "http://localhost:10050"
	c.isolated = "1234567890123456789012345678901234567890"
	c.rawCmd = true

	result, err := c.processTriggerOptions([]string{"arg1", "arg2"})
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string{"arg1", "arg2"}, result.Properties.Command)
	ut.AssertEqual(t, ([]string)(nil), result.Properties.ExtraArgs)
	ut.AssertEqual(t, (*swarming.SwarmingRpcsFilesRef)(nil), result.Properties.InputsRef)
}

func TestProcessTriggerOptions_ExtraArgs(t *testing.T) {
	c := triggerRun{}
	c.Init()
	c.commonFlags.serverURL = "http://localhost:9050"
	c.isolateServer = "http://localhost:10050"
	c.isolated = "1234567890123456789012345678901234567890"

	result, err := c.processTriggerOptions([]string{"arg1", "arg2"})
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string(nil), result.Properties.Command)
	ut.AssertEqual(t, []string{"arg1", "arg2"}, result.Properties.ExtraArgs)
	ut.AssertEqual(t, &swarming.SwarmingRpcsFilesRef{
		Isolated:       "1234567890123456789012345678901234567890",
		Isolatedserver: "http://localhost:10050",
		Namespace:      "default-zip",
	}, result.Properties.InputsRef)
}

func TestProcessTriggerOptions_EatDashDash(t *testing.T) {
	c := triggerRun{}
	c.Init()
	c.commonFlags.serverURL = "http://localhost:9050"
	c.isolateServer = "http://localhost:10050"
	c.isolated = "1234567890123456789012345678901234567890"

	result, err := c.processTriggerOptions([]string{"--", "arg1", "arg2"})
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, []string(nil), result.Properties.Command)
	ut.AssertEqual(t, []string{"arg1", "arg2"}, result.Properties.ExtraArgs)
	ut.AssertEqual(t, &swarming.SwarmingRpcsFilesRef{
		Isolated:       "1234567890123456789012345678901234567890",
		Isolatedserver: "http://localhost:10050",
		Namespace:      "default-zip",
	}, result.Properties.InputsRef)
}
