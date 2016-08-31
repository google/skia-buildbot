// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package authdbimpl implements datastore-based storage and update of AuthDB
// snapshots used for authorization decisions by server/auth/*.
//
// It uses server/auth/service to communicate with auth_service to fetch AuthDB
// snapshots and subscribe to PubSub notifications.
//
// It always uses default datastore namespace for storage, and thus auth groups
// are global to the service.
package authdbimpl
