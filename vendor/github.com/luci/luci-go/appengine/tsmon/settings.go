// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"errors"
	"fmt"
	"html/template"
	"strconv"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/server/settings"
)

// settingsKey is key for tsmon settings (described by tsmonSettings struct)
// in the settings store. See github.com/luci/luci-go/server/settings.
const settingsKey = "tsmon"

// tsmonSettings contain global tsmon settings for the GAE app.
//
// They are stored in app settings store (based on the datastore, see
// appengine/gaesettings module) under settingsKey key.
type tsmonSettings struct {
	// Enabled is false to completely shutoff the monitoring.
	//
	// Default is false.
	Enabled settings.YesOrNo `json:"enabled"`

	// PubsubProject is Cloud Project that hosts metrics ingestion PubSub topic.
	//
	// If not set, metrics will be logged to local GAE log. Default is "".
	PubsubProject string `json:"pubsub_project"`

	// PubsubTopic is a topic inside PubsubProject to send metrics to.
	//
	// If not set, metrics will be logged to local GAE log. Default is "monacq".
	PubsubTopic string `json:"pubsub_topic"`

	// FlushIntervalSec defines how often to flush metrics to the pubsub topic.
	//
	// Default is 60 sec.
	FlushIntervalSec int `json:"flush_interval_sec"`

	// ReportRuntimeStats is true to enable reporting of Go RT stats on flush.
	//
	// Default is false.
	ReportRuntimeStats settings.YesOrNo `json:"report_runtime_stats"`
}

// Prefilled portion of settings.
var defaultSettings = tsmonSettings{
	PubsubProject:    "chrome-infra-mon-pubsub",
	PubsubTopic:      "monacq",
	FlushIntervalSec: 60,
}

// fetchCachedSettings fetches tsmonSettings from the settings store or panics.
//
// Uses in-process global cache to avoid hitting datastore often. The cache
// expiration time is 1 min (see gaesettings.expirationTime), meaning
// the instance will refetch settings once a minute (blocking only one unlucky
// request to do so).
//
// Panics only if there's no cached value (i.e. it is the first call to this
// function in this process ever) and datastore operation fails. It is usually
// very unlikely.
func fetchCachedSettings(c context.Context) tsmonSettings {
	s := tsmonSettings{}
	switch err := settings.Get(c, settingsKey, &s); {
	case err == nil:
		return s
	case err == settings.ErrNoSettings:
		return defaultSettings
	default:
		panic(fmt.Errorf("could not fetch tsmon settings - %s", err))
	}
}

////////////////////////////////////////////////////////////////////////////////
// UI for Tsmon settings.

type settingsUIPage struct {
	settings.BaseUIPage
}

func (settingsUIPage) Title(c context.Context) (string, error) {
	return "Time series monitoring settings", nil
}

func (settingsUIPage) Fields(c context.Context) ([]settings.UIField, error) {
	serviceAcc, err := info.Get(c).ServiceAccount()
	if err != nil {
		return nil, err
	}
	return []settings.UIField{
		settings.YesOrNoField(settings.UIField{
			ID:    "Enabled",
			Title: "Enabled",
			Help: "If not enabled, all metrics manipulations are ignored and the " +
				"monitoring has zero runtime overhead. If enabled, will keep track of metrics " +
				"values in memory and will periodically flush them to PubSub (if PubSub endpoint " +
				"is configured, see below) or GAE log (if not configured).",
		}),
		{
			ID:    "PubsubProject",
			Title: "PubSub project",
			Type:  settings.UIFieldText,
			Help: "Cloud Project that hosts PubSub topic to send metrics to. " +
				"Use <b>chrome-infra-mon-pubsub</b> to send metrics to Chrome Infrastructure " +
				"monitoring pipeline. Keep blank to dump metrics to GAE log instead.",
		},
		{
			ID:    "PubsubTopic",
			Title: "PubSub topic",
			Type:  settings.UIFieldText,
			Help: template.HTML(fmt.Sprintf("Name of the PubSub topic (within the "+
				"project specified above) to send metrics to. The GAE app's service account "+
				"(<b>%s</b>) must have publish permissions for this topic.", serviceAcc)),
		},
		{
			ID:    "FlushIntervalSec",
			Title: "Flush interval, sec",
			Type:  settings.UIFieldText,
			Validator: func(v string) error {
				if i, err := strconv.Atoi(v); err != nil || i < 10 {
					return errors.New("expecting an integer larger than 9")
				}
				return nil
			},
			Help: "How often to flush metrics, in seconds. The default value (60 sec) " +
				"is fine for most cases.",
		},
		settings.YesOrNoField(settings.UIField{
			ID:    "ReportRuntimeStats",
			Title: "Report runtime stats",
			Help: "If enabled, Go runtime state (e.g. memory allocator statistics) " +
				"will be collected at each flush and sent to the monitoring as a bunch " +
				"of go/* metrics.",
		}),
	}, nil
}

func (settingsUIPage) ReadSettings(c context.Context) (map[string]string, error) {
	s := tsmonSettings{}
	switch err := settings.GetUncached(c, settingsKey, &s); {
	case err == settings.ErrNoSettings:
		s = defaultSettings
	case err != nil:
		return nil, err
	}
	return map[string]string{
		"Enabled":            s.Enabled.String(),
		"PubsubProject":      s.PubsubProject,
		"PubsubTopic":        s.PubsubTopic,
		"FlushIntervalSec":   strconv.Itoa(s.FlushIntervalSec),
		"ReportRuntimeStats": s.ReportRuntimeStats.String(),
	}, nil
}

func (settingsUIPage) WriteSettings(c context.Context, values map[string]string, who, why string) error {
	modified := tsmonSettings{}
	modified.PubsubProject = values["PubsubProject"]
	modified.PubsubTopic = values["PubsubTopic"]
	if err := modified.Enabled.Set(values["Enabled"]); err != nil {
		return err
	}
	var err error
	if modified.FlushIntervalSec, err = strconv.Atoi(values["FlushIntervalSec"]); err != nil {
		return err
	}
	if err := modified.ReportRuntimeStats.Set(values["ReportRuntimeStats"]); err != nil {
		return err
	}
	return settings.SetIfChanged(c, settingsKey, &modified, who, why)
}

func init() {
	settings.RegisterUIPage(settingsKey, settingsUIPage{})
}
