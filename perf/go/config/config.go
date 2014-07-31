// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package config

import (
	"time"
)

// QuerySince holds the start time we have data since.
// Don't consider data before this time. May be due to schema changes, etc.
// Note that the limit is exclusive, this date does not contain good data.
type QuerySince time.Time

// TableSuffix returns QuerySince in the BigQuery table suffix format.
func (b QuerySince) BqTableSuffix() string {
	return time.Time(b).Format("20060102")
}

// Date returns QuerySince in the YearMonDay format.
func (b QuerySince) Date() string {
	return time.Time(b).Format("20060102")
}

// GitHashColumn returns QuerySince in the format of SQL table TIMESTAMP
// column.
func (b QuerySince) SqlTsColumn() string {
	return time.Time(b).Format("2006-01-02 15:04:05")
}

func NewQuerySince(t time.Time) QuerySince {
	return QuerySince(t)
}

const (
	// TILE_SCALE The number of points to subsample when moving one level of scaling. I.e.
	// a tile at scale 1 will contain every 4th point of the tiles at scale 0.
	TILE_SCALE = 4

	// The number of samples per trace in a tile, i.e. the number of git hashes that have data
	// in a single tile.
	// TODO fix
	TILE_SIZE = 32

	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100

	// Limit the number of commits we hold in memory and do bulk analysis on.
	MAX_COMMITS_IN_MEMORY = 32

	// Limit the number of times the ingester tries to get a file before giving up.
	MAX_URI_GET_TRIES = 4

	// How often data is refreshed from BigQuery.
	// TODO(jcgregorio) Move to push once it's feasible.
	REFRESH_PERIOD = time.Minute * 30
)

type DatasetName string

const (
	DATASET_SKP   DatasetName = "skps"
	DATASET_MICRO DatasetName = "micro"
)

var (
	ALL_DATASET_NAMES = []DatasetName{DATASET_SKP, DATASET_MICRO}

	// TODO(jcgregorio) Make into a flag.
	BEGINNING_OF_TIME          = QuerySince(time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC))
	HUMAN_READABLE_PARAM_NAMES = map[string]string{
		"antialias":       "Antialiasing",
		"arch":            "CPU Architecture",
		"bbh":             "BBH Setting",
		"benchName":       "SKP Name",
		"builderName":     "Builder Name",
		"config":          "Picture Configuration",
		"configuration":   "Build Configuration",
		"clip":            "Clip",
		"dither":          "Dither",
		"gpu":             "GPU Type",
		"gpuConfig":       "GPU Configuration",
		"measurementType": "Measurement Type",
		"mode":            "Mode Configuration",
		"model":           "Buildbot Model",
		"os":              "OS",
		"role":            "Buildbot Role",
		"rotate":          "Rotate",
		"scale":           "Scale Setting",
		"skpSize":         "SKP Size",
		"system":          "System Type",
		"testName":        "Test Name",
		"viewport":        "Viewport Size",
	}
	// TODO: Make sure these are sufficient for a key
	KEY_PARAM_ORDER = map[DatasetName][]string{
		DATASET_SKP:   []string{"builderName", "benchName", "config", "scale", "measurementType", "configuration", "mode"},
		DATASET_MICRO: []string{"builderName", "testName", "config", "scale", "measurementType"},
	}
)
