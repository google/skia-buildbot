// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbucket

import (
	"bytes"
	"strconv"
)

// This file is about buildbucket build params in the format supported by
// Buildbot.

// properties is well-established build properties.
type properties struct {
	PatchStorage string `json:"patch_storage"` // e.g. "rietveld", "gerrit"

	RietveldURL string `json:"rietveld"` // e.g. "https://codereview.chromium.org"
	Issue       number `json:"issue"`    // e.g. 2127373005
	PatchSet    number `json:"patchset"` // e.g. 40001

	GerritURL          string `json:"gerrit"`              // e.g. "https://chromium-review.googlesource.com"
	GerritChangeNumber int    `json:"event.change.number"` // e.g. 358171
	GerritChangeID     string `json:"event.change.id"`     // e.g. "infra%2Finfra~master~Iee05b76799d577d491f533b8acaa4560ac14a806"
	GerritChangeURL    string `json:"event.change.url"`    // e.g. "https://chromium-review.googlesource.com/#/c/358171"
	GerritPatchRef     string `json:"event.patchSet.ref"`  // e.g. "refs/changes/71/358171/2"

	Revision  string   `json:"revision"`  // e.g. "0b04861933367c62630751702c84fd64bc3caf6f"
	BlameList []string `json:"blamelist"` // e.g. ["someone@chromium.org"]

	// Fields below are present only in ResultDetails.

	GotRevision string `json:"got_revision"` // e.g. "0b04861933367c62630751702c84fd64bc3caf6f"
	BuildNumber int    `json:"buildnumber"`  // e.g. 3021
}

// number is an integer that supports JSON unmarshalling from a string.
type number int

// UnmarshalJSON parses data as an integer, whether data is a number or string.
func (n *number) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, `"`)
	num, err := strconv.Atoi(string(data))
	if err == nil {
		*n = number(num)
	}
	return err
}

// change is used in "changes" buildbucket parameters; supported by buildbot
// See https://chromium.googlesource.com/chromium/tools/build/+/master/scripts/master/buildbucket/README.md#Build-parameters
type change struct {
	Author struct{ Email string }
}

// buildParameters is contents of "parameters_json" buildbucket build field
// in the format supported by Buildbot, see
// // See https://chromium.googlesource.com/chromium/tools/build/+/master/scripts/master/buildbucket/README.md#Build-parameters
//
// Buildbucket is not aware of this format, but majority of chrome-infra is.
type buildParameters struct {
	BuilderName string `json:"builder_name"`
	Properties  properties
	Changes     []change
}

// resultDetails is contents of "result_details_json" buildbucket build field
// in the format supported by Buildbot.
type resultDetails struct {
	Properties properties
}
