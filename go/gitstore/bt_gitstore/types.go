// Copyright 2019 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bt_gitstore

// BTConfig contains the BigTable configuration to define where the repo should be stored.
type BTConfig struct {
	ProjectID       string
	InstanceID      string
	TableID         string
	AppProfile      string
	Shards          int
	WriteGoroutines int
}
