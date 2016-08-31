// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import "github.com/luci/luci-go/server/auth/identity"

// UserConfig keys off of an identity and stores user configuration.
type UserConfig struct {
	UserID identity.Identity `gae:"$id"`
	Theme  string
}
