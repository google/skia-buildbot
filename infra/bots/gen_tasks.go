// Copyright 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

/*
	Generate the tasks.json file.
*/

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	DEFAULT_OS = "Ubuntu"

	// Pool for Skia bots.
	POOL_SKIA = "Skia"
)

var (
	// "Constants"

	// Top-level list of all Jobs to run at each commit.
	JOBS = []string{
		"Infra-PerCommit",
	}

	// Flags.
	testing = flag.Bool("test", false, "Run in test mode: verify that the output hasn't changed.")
)

// infra generates an infra test Task. Returns the name of the last Task in the
// generated chain of Tasks, which the Job should add as a dependency.
func infra(cfg *specs.TasksCfg, name string) string {
	// Add the Task.
	cfg.Tasks[name] = &specs.TaskSpec{
		CipdPackages: []*specs.CipdPackage{},
		Dimensions: []string{
			"pool:Skia",
			"os:Ubuntu",
			"gpu:none",
		},
		ExtraArgs: []string{
			"--workdir", "../../..", "swarm_infra",
			fmt.Sprintf("repository=%s", specs.PLACEHOLDER_REPO),
			fmt.Sprintf("buildername=%s", name),
			"mastername=fake-master",
			"buildnumber=2",
			"slavename=fake-buildslave",
			"nobuildbot=True",
			fmt.Sprintf("swarm_out_dir=%s", specs.PLACEHOLDER_ISOLATED_OUTDIR),
			fmt.Sprintf("revision=%s", specs.PLACEHOLDER_REVISION),
			fmt.Sprintf("patch_storage=%s", specs.PLACEHOLDER_PATCH_STORAGE),
			fmt.Sprintf("event.change.number=%s", specs.PLACEHOLDER_ISSUE),
			fmt.Sprintf("event.patchSet.ref=refs/changes/%s/%s/%s", specs.PLACEHOLDER_ISSUE_SHORT, specs.PLACEHOLDER_ISSUE, specs.PLACEHOLDER_PATCHSET),
		},
		Isolate:  "swarm_recipe.isolate",
		Priority: 0.8,
	}
	return name
}

// process generates Tasks and Jobs for the given Job name.
func process(cfg *specs.TasksCfg, name string) {
	if _, ok := cfg.Jobs[name]; ok {
		glog.Fatalf("Duplicate Job %q", name)
	}
	deps := []string{}

	// Infra tests.
	if name == "Infra-PerCommit" {
		deps = append(deps, infra(cfg, name))
	}

	// Add the Job spec.
	cfg.Jobs[name] = &specs.JobSpec{
		Priority:  0.8,
		TaskSpecs: deps,
	}
}

// getCheckoutRoot returns the path of the root of the checkout.
func getCheckoutRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		glog.Fatal(err)
	}
	for {
		if _, err := os.Stat(cwd); err != nil {
			glog.Fatal(err)
		}
		s, err := os.Stat(path.Join(cwd, ".git"))
		if err == nil && s.IsDir() {
			// TODO(borenet): Should we verify that this is the
			// Skia infra checkout and not something else?
			return cwd
		}
		cwd = filepath.Clean(path.Join(cwd, ".."))
	}
}

// Regenerate the tasks.json file.
func main() {
	common.Init()
	defer common.LogPanic()

	// Where are we?
	root := getCheckoutRoot()

	// Create the config.
	cfg := &specs.TasksCfg{
		Jobs:  map[string]*specs.JobSpec{},
		Tasks: map[string]*specs.TaskSpec{},
	}

	// Create Tasks and Jobs.
	for _, j := range JOBS {
		process(cfg, j)
	}

	// Validate the config.
	if err := cfg.Validate(); err != nil {
		glog.Fatal(err)
	}

	// Write the tasks.json file.
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		glog.Fatal(err)
	}
	// The json package escapes HTML characters, which makes our output
	// much less readable. Replace the escape characters with the real
	// character.
	b = bytes.Replace(b, []byte("\\u003c"), []byte("<"), -1)

	outFile := path.Join(root, specs.TASKS_CFG_FILE)
	if *testing {
		// Don't write the file; read it and compare.
		expect, err := ioutil.ReadFile(outFile)
		if err != nil {
			glog.Fatal(err)
		}
		if !bytes.Equal(expect, b) {
			glog.Fatalf("Expected no changes, but changes were found!")
		}
	} else {
		if err := ioutil.WriteFile(outFile, b, os.ModePerm); err != nil {
			glog.Fatal(err)
		}
	}
}
