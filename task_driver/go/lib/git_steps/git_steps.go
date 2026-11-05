// Copyright 2019 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package git_steps

/*
   Task Driver utilities for working with Git.
*/

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/task_driver/go/lib/auth_steps"
	"golang.org/x/oauth2"
)

// Init initializes git auth for a Task Driver. Returns a TokenSource or any
// error which occurred.
func Init(ctx context.Context, local bool) (oauth2.TokenSource, error) {
	return auth_steps.Init(ctx, local, auth.ScopeGerrit, auth.ScopeUserinfoEmail)
}
