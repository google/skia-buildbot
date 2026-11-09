// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

// Persona identifies a pre-configured Perf instance profile.
type Persona string

const (
	PersonaChrome Persona = "chrome"
	PersonaV8     Persona = "v8"
)

// PersonaConfig serializes to the window.perf configuration object injected
// into HTML pages served by the mock frontend.
type PersonaConfig struct {
	InstanceURL                 string   `json:"instance_url"`
	InstanceName                string   `json:"instance_name"`
	HeaderImageURL              string   `json:"header_image_url"`
	CommitRangeURL              string   `json:"commit_range_url"`
	KeyOrder                    []string `json:"key_order"`
	Demo                        bool     `json:"demo"`
	Radius                      int      `json:"radius"`
	NumShift                    int      `json:"num_shift"`
	Interesting                 float64  `json:"interesting"`
	StepUpOnly                  bool     `json:"step_up_only"`
	DisplayGroupBy              bool     `json:"display_group_by"`
	HideListOfCommitsOnExplore  bool     `json:"hide_list_of_commits_on_explore"`
	Notifications               string   `json:"notifications"`
	FetchChromePerfAnomalies    bool     `json:"fetch_chrome_perf_anomalies"`
	FetchAnomaliesFromSQL       bool     `json:"fetch_anomalies_from_sql"`
	FeedbackURL                 string   `json:"feedback_url"`
	ChatURL                     string   `json:"chat_url"`
	HelpURLOverride             string   `json:"help_url_override"`
	TraceFormat                 string   `json:"trace_format"`
	NeedAlertAction             bool     `json:"need_alert_action"`
	BugHostURL                  string   `json:"bug_host_url"`
	GitRepoURL                  string   `json:"git_repo_url"`
	KeysForCommitRange          []string `json:"keys_for_commit_range"`
	KeysForUsefulLinks          []string `json:"keys_for_useful_links"`
	SkipCommitDetailDisplay     bool     `json:"skip_commit_detail_display"`
	ImageTag                    string   `json:"image_tag"`
	RemoveDefaultStatValue      bool     `json:"remove_default_stat_value"`
	EnableSkiaBridgeAggregation bool     `json:"enable_skia_bridge_aggregation"`
	ShowJsonFileDisplay         bool     `json:"show_json_file_display"`
	ShowTriageLink              bool     `json:"show_triage_link"`
	ShowBisectBtn               bool     `json:"show_bisect_btn"`
	AppVersion                  string   `json:"app_version"`
	EnableV2UI                  bool     `json:"enable_v2_ui"`
	DevMode                     bool     `json:"dev_mode"`
	DefaultToManualPlotMode     bool     `json:"default_to_manual_plot_mode"`
}

// ConfigForPersona returns the window.perf configuration for the requested
// persona. Chrome sets git_repo_url to chromium/src and show_bisect_btn to true
// so chart-tooltip-sk renders both Pinpoint buttons; V8 points git_repo_url to
// v8.git and sets show_bisect_btn to false so both buttons stay hidden.
func ConfigForPersona(persona Persona) PersonaConfig {
	base := PersonaConfig{
		InstanceURL:                 "https://chrome-perf.corp.goog",
		InstanceName:                "chrome-perf-mock",
		HeaderImageURL:              "",
		CommitRangeURL:              "https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}",
		KeyOrder:                    []string{"benchmark", "bot", "test", "subtest_1", "subtest_2"},
		Demo:                        true,
		Radius:                      7,
		NumShift:                    10,
		Interesting:                 25,
		StepUpOnly:                  false,
		DisplayGroupBy:              true,
		HideListOfCommitsOnExplore:  false,
		Notifications:               "none",
		FetchChromePerfAnomalies:    true,
		FetchAnomaliesFromSQL:       true,
		FeedbackURL:                 "http://feedback",
		ChatURL:                     "http://chat",
		HelpURLOverride:             "http://help",
		TraceFormat:                 "chrome",
		NeedAlertAction:             false,
		BugHostURL:                  "https://issues.chromium.org",
		GitRepoURL:                  "https://chromium.googlesource.com/chromium/src",
		KeysForCommitRange:          []string{},
		KeysForUsefulLinks:          []string{"Build Page"},
		SkipCommitDetailDisplay:     false,
		ImageTag:                    "mock-tag",
		RemoveDefaultStatValue:      false,
		EnableSkiaBridgeAggregation: false,
		ShowJsonFileDisplay:         false,
		ShowTriageLink:              true,
		ShowBisectBtn:               true,
		AppVersion:                  "mock-v1",
		EnableV2UI:                  true,
		DevMode:                     true,
		DefaultToManualPlotMode:     false,
	}

	if persona == PersonaV8 {
		base.InstanceURL = "https://v8-perf.corp.goog"
		base.InstanceName = "v8-perf-mock"
		base.CommitRangeURL = "https://chromium.googlesource.com/v8/v8.git/+log/{begin}..{end}"
		base.GitRepoURL = "https://chromium.googlesource.com/v8/v8.git"
		base.ShowBisectBtn = false
	}

	return base
}
