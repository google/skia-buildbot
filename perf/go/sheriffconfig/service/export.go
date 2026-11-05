// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package service

import (
	"go.skia.org/infra/perf/go/alerts"
	pb "go.skia.org/infra/perf/go/sheriffconfig/proto/v1"
)

// SubscriptionToAlerts exports the makeSaveRequests logic, directly returning a slice of alerts.Alert.
func SubscriptionToAlerts(subscription *pb.Subscription) ([]*alerts.Alert, error) {
	saveReqs, err := makeSaveRequests(subscription, "dry-run")
	if err != nil {
		return nil, err
	}
	var res []*alerts.Alert
	for _, sr := range saveReqs {
		res = append(res, sr.Cfg)
	}
	return res, nil
}
