// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/common/logging/memlogger"
	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/metric"
	"github.com/luci/luci-go/common/tsmon/types"

	"github.com/luci/luci-go/tokenserver/api"
	"github.com/luci/luci-go/tokenserver/client"
)

// UpdateOutcome describes overall status of tokend token update process.
type UpdateOutcome string

// Some known outcomes.
//
// See also OutcomeFromRPCError for outcomes generated from status codes.
const (
	OutcomeTokenIsGood           UpdateOutcome = "TOKEN_IS_GOOD"  // token is still valid
	OutcomeUpdateSuccess         UpdateOutcome = "UPDATE_SUCCESS" // successfully updated
	OutcomeCantReadKey           UpdateOutcome = "CANT_READ_KEY"
	OutcomeMalformedReponse      UpdateOutcome = "MALFORMED_RESPONSE"
	OutcomeUnknownRPCError       UpdateOutcome = "UNKNOWN_RPC_ERROR"
	OutcomePermissionError       UpdateOutcome = "SAVE_TOKEN_PERM_ERROR"
	OutcomeUnknownSaveTokenError UpdateOutcome = "UNKNOWN_SAVE_TOKEN_ERROR"
)

// OutcomeFromRPCError transform MintToken error into an update outcome.
func OutcomeFromRPCError(err error) UpdateOutcome {
	if err == nil {
		return OutcomeUpdateSuccess
	}
	if details, ok := err.(client.RPCError); ok {
		if details.GrpcCode != codes.OK {
			return UpdateOutcome(fmt.Sprintf("GRPC_ERROR_%d", details.GrpcCode))
		}
		return UpdateOutcome(fmt.Sprintf("MINT_TOKEN_ERROR_%s", details.ErrorCode))
	}
	return OutcomeUnknownRPCError
}

// UpdateReason describes why tokend attempts to update the token.
type UpdateReason string

// All known reasons for starting token refresh procedure.
const (
	UpdateReasonTokenIsGood      UpdateReason = "TOKEN_IS_GOOD" // update was skipped
	UpdateReasonNewToken         UpdateReason = "NEW_TOKEN"
	UpdateReasonExpiration       UpdateReason = "TOKEN_EXPIRES"
	UpdateReasonParametersChange UpdateReason = "PARAMS_CHANGE"
	UpdateReasonForceRefresh     UpdateReason = "FORCE_REFRESH"
)

// StatusReport gathers information about tokend run.
//
// It is picked up by monitoring harness later.
type StatusReport struct {
	Version           string                 // major version of the tokend executable
	Started           time.Time              // when the process started
	Finished          time.Time              // when the process finished
	UpdateOutcome     UpdateOutcome          // overall outcome of the token update process
	UpdateReason      UpdateReason           // why tokend attempts to update the token
	FailureError      error                  // immediate error that caused the failure
	MintTokenDuration time.Duration          // how long RPC call lasted (with all retries)
	LastToken         *tokenserver.TokenFile // last known token (possibly refreshed)
	ServiceVersion    string                 // name and version of the server that generated the token
}

// Report is how status report looks on disk.
type Report struct {
	TokendVersion     string `json:"tokend_version"`
	ServiceVersion    string `json:"service_version,omitempty"`
	StartedTS         int64  `json:"started_ts"`
	TotalDuration     int64  `json:"total_duration_us,omitempty"`
	RPCDuration       int64  `json:"rpc_duration_us,omitempty"`
	UpdateOutcome     string `json:"update_outcome,omitempty"`
	UpdateReason      string `json:"update_reason,omitempty"`
	FailureError      string `json:"failure_error,omitempty"`
	LogDump           string `json:"log_dump"`
	TokenLastUpdateTS int64  `json:"token_last_update_ts,omitempty"`
	TokenNextUpdateTS int64  `json:"token_next_update_ts,omitempty"`
	TokenExpiryTS     int64  `json:"token_expiry_ts,omitempty"`
}

// Report gathers the report into single JSON-serializable struct.
func (s *StatusReport) Report() *Report {
	rep := &Report{
		TokendVersion:  s.Version,
		ServiceVersion: s.ServiceVersion,
		StartedTS:      s.Started.Unix(),
		TotalDuration:  s.Finished.Sub(s.Started).Nanoseconds() / 1000,
		RPCDuration:    s.MintTokenDuration.Nanoseconds() / 1000,
		UpdateOutcome:  string(s.UpdateOutcome),
		UpdateReason:   string(s.UpdateReason),
	}
	if s.FailureError != nil {
		rep.FailureError = s.FailureError.Error()
	}
	if s.LastToken != nil {
		rep.TokenLastUpdateTS = s.LastToken.LastUpdate
		rep.TokenNextUpdateTS = s.LastToken.NextUpdate
		rep.TokenExpiryTS = s.LastToken.Expiry
	}
	return rep
}

// SaveToFile saves the status report and log to a file on disk.
func (s *StatusReport) SaveToFile(ctx context.Context, l *memlogger.MemLogger, path string) error {
	report := s.Report()

	buf := bytes.Buffer{}
	l.Dump(&buf)
	report.LogDump = buf.String()

	blob, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(ctx, path, blob, 0644)
}

////////////////////////////////////////////////////////////////////////////////
// All tsmon metrics.

var (
	// E.g. "1.0". See Version const in main.go.
	metricVersion = metric.NewString(
		"luci/machine_tokend/version",
		"Major version of luci_machine_tokend executable",
		types.MetricMetadata{})

	// E.g. "luci-token-server/2123-abcdef" (<appid>/<version>).
	metricServiceVersion = metric.NewString(
		"luci/machine_tokend/service_version",
		"Identifier of the server version that generated the token",
		types.MetricMetadata{})

	// This should be >=30 min in the future if everything is ok. If update
	// process fails repeatedly, it will be in the past (and the token is unusable
	// at this point).
	metricTokenExpiry = metric.NewInt(
		"luci/machine_tokend/token_expiry_ts",
		"Unix timestamp of when the token expires, in microsec",
		types.MetricMetadata{Units: types.Microseconds})

	// This should be no longer than 30 min in the past if everything is ok.
	metricTokenLastUpdate = metric.NewInt(
		"luci/machine_tokend/last_update_ts",
		"Unix timestamp of when the token was successfully updated, in microsec",
		types.MetricMetadata{Units: types.Microseconds})

	// This should be [0-30] min in the future if everything ok. If update process
	// fails (at least once), it will be in the past. It's not a fatal condition
	// yet.
	metricTokenNextUpdate = metric.NewInt(
		"luci/machine_tokend/next_update_ts",
		"Unix timestamp of when the token must be updated next time, in microsec",
		types.MetricMetadata{Units: types.Microseconds})

	// See UpdateOutcome enum and OutcomeFromRPCError for possible values.
	//
	// Positive values are "TOKEN_IS_GOOD" and "UPDATE_SUCCESS".
	metricUpdateOutcome = metric.NewString(
		"luci/machine_tokend/update_outcome",
		"Overall outcome of the luci_machine_tokend invocation",
		types.MetricMetadata{})

	// See UpdateReason enum for possible values.
	metricUpdateReason = metric.NewString(
		"luci/machine_tokend/update_reason",
		"Why the token was updated or 'TOKEN_IS_GOOD' if token is still valid",
		types.MetricMetadata{})

	metricTotalDuration = metric.NewInt(
		"luci/machine_tokend/duration_total_us",
		"For how long luci_machine_tokend ran (including all local IO) in microsec",
		types.MetricMetadata{Units: types.Microseconds})

	metricRPCDuration = metric.NewInt(
		"luci/machine_tokend/duration_rpc_us",
		"For how long an RPC to backend ran in microsec",
		types.MetricMetadata{Units: types.Microseconds})
)

// SendMetrics is called at the end of the token update process.
//
// It dumps all relevant metrics to tsmon.
func (s *StatusReport) SendMetrics(c context.Context) error {
	c, _ = context.WithTimeout(c, 10*time.Second)
	rep := s.Report()

	metricVersion.Set(c, rep.TokendVersion)
	if rep.ServiceVersion != "" {
		metricServiceVersion.Set(c, rep.ServiceVersion)
	}
	if rep.TokenExpiryTS != 0 {
		metricTokenExpiry.Set(c, rep.TokenExpiryTS*1000000)
	}
	if rep.TokenLastUpdateTS != 0 {
		metricTokenLastUpdate.Set(c, rep.TokenLastUpdateTS*1000000)
	}
	if rep.TokenNextUpdateTS != 0 {
		metricTokenNextUpdate.Set(c, rep.TokenNextUpdateTS*1000000)
	}
	metricUpdateOutcome.Set(c, rep.UpdateOutcome)
	metricUpdateReason.Set(c, rep.UpdateReason)
	metricTotalDuration.Set(c, rep.TotalDuration)
	metricRPCDuration.Set(c, rep.RPCDuration)

	return tsmon.Flush(c)
}
