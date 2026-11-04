// Copyright 2016 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package metrics2

// GetCounter creates and returns a new Counter using the default client.
func GetCounter(name string, tags ...map[string]string) Counter {
	return defaultClient.GetCounter(name, tags...)
}
