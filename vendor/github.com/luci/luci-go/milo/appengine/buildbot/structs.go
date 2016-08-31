// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/milo/api/resp"
)

// This file contains all of the structs that define buildbot json endpoints.
// This is primarily used for unmarshalling buildbot master and build json.
// json.UnmarshalJSON can directly unmarshal buildbot jsons into these structs.
// Many of the structs were initially built using https://mholt.github.io/json-to-go/

// buildbotStep represents a single step in a buildbot build.
type buildbotStep struct {
	// We actually don't care about ETA.  This is represented as a string if
	// it's fetched from a build json, but a float if it's dug out of the
	// slave portion of a master json.  We'll just set it to interface and
	// ignore it.
	Eta          interface{}     `json:"eta"`
	Expectations [][]interface{} `json:"expectations"`
	Hidden       bool            `json:"hidden"`
	Isfinished   bool            `json:"isFinished"`
	Isstarted    bool            `json:"isStarted"`
	Logs         [][]string      `json:"logs"`
	Name         string          `json:"name"`
	Results      []interface{}   `json:"results"`
	Statistics   struct {
	} `json:"statistics"`
	StepNumber int        `json:"step_number"`
	Text       []string   `json:"text"`
	Times      []*float64 `json:"times"`
	Urls       struct {
	} `json:"urls"`
}

// buildbotSourceStamp is a list of changes (commits) tagged with where the changes
// came from, ie. the project/repository.  Also includes a "main" revision."
type buildbotSourceStamp struct {
	Branch     *string          `json:"branch"`
	Changes    []buildbotChange `json:"changes"`
	Haspatch   bool             `json:"hasPatch"`
	Project    string           `json:"project"`
	Repository string           `json:"repository"`
	Revision   string           `json:"revision"`
}

// buildbotBuild is a single build json on buildbot.
type buildbotBuild struct {
	Master      string   `gae:"$master"`
	Blame       []string `json:"blame" gae:"-"`
	Buildername string   `json:"builderName"`
	// This needs to be reflected.  This can be either a String or a buildbotStep.
	Currentstep interface{} `json:"currentStep" gae:"-"`
	// We don't care about this one.
	Eta    interface{} `json:"eta" gae:"-"`
	Logs   [][]string  `json:"logs" gae:"-"`
	Number int         `json:"number"`
	// This is a slice of tri-tuples of [property name, value, source].
	// property name is always a string
	// value can be a string or float
	// source is optional, but is always a string if present
	Properties  [][]interface{}      `json:"properties" gae:"-"`
	Reason      string               `json:"reason"`
	Results     *int                 `json:"results" gae:"-"`
	Slave       string               `json:"slave"`
	Sourcestamp *buildbotSourceStamp `json:"sourceStamp" gae:"-"`
	Steps       []buildbotStep       `json:"steps" gae:"-"`
	Text        []string             `json:"text" gae:"-"`
	Times       []*float64           `json:"times" gae:"-"`
	// This one is injected by the publisher module.  Does not exist in a
	// normal json query.
	TimeStamp *int `json:"timeStamp" gae:"-"`
	// This one is marked by Milo, denotes whether or not the build is internal.
	Internal bool `json:"internal" gae:"-"`
	// This one is computed by Milo for indexing purposes.  It does so by
	// checking to see if times[1] is null or not.
	Finished bool `json:"finished"`
	// OS is a string representation of the OS the build ran on.  This is
	// derived best-effort from the slave information in the master JSON.
	// This information is injected into the buildbot builds via puppet, and
	// comes as Family + Version.  Family is (windows, Darwin, Debian), while
	// Version is the version of the OS, such as (XP, 7, 10) for windows.
	OSFamily  string `json:"osFamily"`
	OSVersion string `json:"osVersion"`
}

func (b *buildbotBuild) toStatus() resp.Status {
	var result resp.Status
	if b.Currentstep != nil {
		result = resp.Running
	} else {
		result = result2Status(b.Results)
	}
	return result
}

var _ datastore.PropertyLoadSaver = (*buildbotBuild)(nil)
var _ datastore.MetaGetterSetter = (*buildbotBuild)(nil)

// getID is a helper function that returns the datastore key for a given
// build.
func (b *buildbotBuild) getID() string {
	s := []string{b.Master, b.Buildername, strconv.Itoa(b.Number)}
	id, err := json.Marshal(s)
	if err != nil {
		panic(err) // This really shouldn't fail.
	}
	return string(id)
}

// setKeys is the inverse of getID().
func (b *buildbotBuild) setKeys(id string) error {
	s := []string{}
	err := json.Unmarshal([]byte(id), &s)
	if err != nil {
		return err
	}
	if len(s) != 3 {
		return fmt.Errorf("%s does not have 3 items", id)
	}
	b.Master = s[0]
	b.Buildername = s[1]
	b.Number, err = strconv.Atoi(s[2])
	return err // or nil.
}

// GetMeta is overridden so that a query for "id" calls getID() instead of
// the superclass method.
func (b *buildbotBuild) GetMeta(key string) (interface{}, bool) {
	if key == "id" {
		if b.Master == "" || b.Buildername == "" {
			panic(fmt.Errorf("No Master or Builder found"))
		}
		return b.getID(), true
	}
	return datastore.GetPLS(b).GetMeta(key)
}

// GetAllMeta is overridden for the same reason GetMeta() is.
func (b *buildbotBuild) GetAllMeta() datastore.PropertyMap {
	p := datastore.GetPLS(b).GetAllMeta()
	p.SetMeta("id", b.getID())
	return p
}

// SetMeta is the inverse of GetMeta().
func (b *buildbotBuild) SetMeta(key string, val interface{}) bool {
	if key == "id" {
		err := b.setKeys(val.(string))
		if err != nil {
			panic(err)
		}
	}
	return datastore.GetPLS(b).SetMeta(key, val)
}

// Load translates a propertymap into the struct and loads the data into
// the struct.
func (b *buildbotBuild) Load(p datastore.PropertyMap) error {
	gz, err := p["data"][0].Project(datastore.PTBytes)
	if err != nil {
		return err
	}
	reader, err := gzip.NewReader(bytes.NewReader(gz.([]byte)))
	if err != nil {
		return err
	}
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	return json.Unmarshal(bs, b)
}

var errTooBig = errors.New("buildbotBuild: Build is too big to save.")

// Save returns a property map of the struct to save in the datastore.  It
// contains two fields, the ID which is the key, and a data field which is a
// serialized and gzipped representation of the entire struct.
func (b *buildbotBuild) Save(withMeta bool) (datastore.PropertyMap, error) {
	bs, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	gzbs := bytes.Buffer{}
	gsw := gzip.NewWriter(&gzbs)
	_, err = gsw.Write(bs)
	if err != nil {
		return nil, err
	}
	err = gsw.Close()
	if err != nil {
		return nil, err
	}
	blob := gzbs.Bytes()
	// Datastore has a max size of 1MB.  If the blob is over 9.5MB, it probably
	// won't fit after accounting for overhead.
	if len(blob) > 950000 {
		return nil, errTooBig
	}
	p := datastore.PropertyMap{
		"data": []datastore.Property{
			datastore.MkPropertyNI(blob),
		},
	}
	if withMeta {
		p["id"] = []datastore.Property{
			datastore.MkPropertyNI(b.getID()),
		}
		p["master"] = []datastore.Property{
			datastore.MkProperty(b.Master),
		}
		p["builder"] = []datastore.Property{
			datastore.MkProperty(b.Buildername),
		}
		p["number"] = []datastore.Property{
			datastore.MkProperty(b.Number),
		}
		p["finished"] = []datastore.Property{
			datastore.MkProperty(b.Finished),
		}
	}
	return p, nil
}

// buildbotBuilder is a builder struct from the master json, _not_ the builder json.
type buildbotBuilder struct {
	Basedir       string   `json:"basedir"`
	Cachedbuilds  []int    `json:"cachedBuilds"`
	Category      string   `json:"category"`
	Currentbuilds []int    `json:"currentBuilds"`
	Slaves        []string `json:"slaves"`
	State         string   `json:"state"`
}

// buildbotChangeSource is a changesource (ie polling source) usually tied to a master's scheduler.
type buildbotChangeSource struct {
	Description string `json:"description"`
}

// buildbotChange describes a commit in a repository as part of a changesource of blamelist.
type buildbotChange struct {
	At       string  `json:"at"`
	Branch   *string `json:"branch"`
	Category string  `json:"category"`
	Comments string  `json:"comments"`
	// This could be a list of strings or list of struct { Name string } .
	Files      []interface{}   `json:"files"`
	Number     int             `json:"number"`
	Project    string          `json:"project"`
	Properties [][]interface{} `json:"properties"`
	Repository string          `json:"repository"`
	Rev        string          `json:"rev"`
	Revision   string          `json:"revision"`
	Revlink    string          `json:"revlink"`
	When       int             `json:"when"`
	Who        string          `json:"who"`
}

// buildbotSlave describes a slave on a master from a master json, and also includes the
// full builds of any currently running builds.
type buildbotSlave struct {
	// RecentBuilds is a map of builder name to a list of recent finished build
	// numbers on that builder.
	RecentBuilds  map[string][]int `json:"builders"`
	Connected     bool             `json:"connected"`
	Host          string           `json:"host"`
	Name          string           `json:"name"`
	Runningbuilds []buildbotBuild  `json:"runningBuilds"`
	Version       string           `json:"version"`
	// This is like runningbuilds, but instead of storing the full build,
	// just reference the build by builder: build num.
	RunningbuildsMap map[string][]int `json:"runningBuildsMap"`
}

type buildbotProject struct {
	BuildbotURL string `json:"buildbotURL"`
	Title       string `json:"title"`
	Titleurl    string `json:"titleURL"`
}

// buildbotMaster This is json definition for https://build.chromium.org/p/<master>/json
// endpoints.
type buildbotMaster struct {
	AcceptingBuilds struct {
		AcceptingBuilds bool `json:"accepting_builds"`
	} `json:"accepting_builds"`

	Builders map[string]buildbotBuilder `json:"builders"`

	Buildstate struct {
		AcceptingBuilds bool `json:"accepting_builds"`
		Builders        []struct {
			Basedir       string   `json:"basedir"`
			Buildername   string   `json:"builderName"`
			Cachedbuilds  []int    `json:"cachedBuilds"`
			Category      string   `json:"category"`
			Currentbuilds []int    `json:"currentBuilds"`
			Slaves        []string `json:"slaves"`
			State         string   `json:"state"`
		} `json:"builders"`
		Project struct {
			BuildbotURL string `json:"buildbotURL"`
			Title       string `json:"title"`
			Titleurl    string `json:"titleURL"`
		} `json:"project"`
		Timestamp float64 `json:"timestamp"`
	} `json:"buildstate"`

	ChangeSources map[string]buildbotChangeSource `json:"change_sources"`

	Changes map[string]buildbotChange `json:"changes"`

	Clock struct {
		Current struct {
			Local string  `json:"local"`
			Utc   string  `json:"utc"`
			UtcTs float64 `json:"utc_ts"`
		} `json:"current"`
		ServerStarted struct {
			Local string  `json:"local"`
			Utc   string  `json:"utc"`
			UtcTs float64 `json:"utc_ts"`
		} `json:"server_started"`
		ServerUptime float64 `json:"server_uptime"`
	} `json:"clock"`

	Project buildbotProject `json:"project"`

	Slaves map[string]*buildbotSlave `json:"slaves"`

	Varz struct {
		AcceptingBuilds bool `json:"accepting_builds"`
		Builders        map[string]struct {
			ConnectedSlaves int    `json:"connected_slaves"`
			CurrentBuilds   int    `json:"current_builds"`
			PendingBuilds   int    `json:"pending_builds"`
			State           string `json:"state"`
			TotalSlaves     int    `json:"total_slaves"`
		} `json:"builders"`
		ServerUptime float64 `json:"server_uptime"`
	} `json:"varz"`

	// This is injected by the pubsub publisher on the buildbot side.
	Name string `json:"name"`
}
