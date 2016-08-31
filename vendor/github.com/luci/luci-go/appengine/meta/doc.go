// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package meta contains some methods for interacting with GAE's metadata APIs.
// It only contains an implementation for those metadata APIs we've needed so
// far, but should be extended to support new ones in the case that we use them.
//
// See metadata docs: https://cloud.google.com/appengine/docs/python/datastore/metadataentityclasses#EntityGroup
package meta
