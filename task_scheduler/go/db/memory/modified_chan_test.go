// Copyright 2019 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"testing"

	"go.skia.org/infra/task_scheduler/go/db"
)

func TestModifiedTasksCh(t *testing.T) {
	d := NewInMemoryDB()
	db.TestModifiedTasksCh(t, d)
}

func TestModifiedJobsCh(t *testing.T) {
	d := NewInMemoryDB()
	db.TestModifiedJobsCh(t, d)
}

func TestModifiedTaskCommentsCh(t *testing.T) {
	d := NewInMemoryDB()
	db.TestModifiedTaskCommentsCh(t, d)
}

func TestModifiedTaskSpecCommentsCh(t *testing.T) {
	d := NewInMemoryDB()
	db.TestModifiedTaskSpecCommentsCh(t, d)
}

func TestModifiedCommitCommentsCh(t *testing.T) {
	d := NewInMemoryDB()
	db.TestModifiedCommitCommentsCh(t, d)
}
