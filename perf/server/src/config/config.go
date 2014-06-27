// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package config

import "time"

// BeginningOfTime holds the start time we have data since.
type BeginningOfTime time.Time

// TableSuffix returns BeginningOfTime in the BigQuery table suffix format.
func (b BeginningOfTime)BqTableSuffix() string {
	return time.Time(b).Format("20060102")
}

// GitHashColumn returns BeginningOfTime in the format of SQL table TIMESTAMP
// column.
func (b BeginningOfTime)SqlTsColumn() string {
	return time.Time(b).Format("2006-01-02 15:04:05")
}

const (
	// JSON doesn't support NaN or +/- Inf, so we need a valid float
	// to signal missing data that also has a compact JSON representation.
	MISSING_DATA_SENTINEL = 1e100

	// Don't consider data before this time. May be due to schema changes, etc.
	// Note that the limit is exclusive, this date does not contain good data.
	// Limit the number of commits we hold in memory and do bulk analysis on.
	MAX_COMMITS_IN_MEMORY = 30

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
	BEGINNING_OF_TIME = BeginningOfTime(time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC))
)
