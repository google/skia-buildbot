// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package analytics provides a standard way to store the Google Analytics
// tracking ID.
//
// Usage Example
//
// This analytics package provides a convenient way to configure an analytics
// ID for an app.  To use, just call analytics.ID(c) and inject that ID
// into a template.
//
//   import (
//		 ...
//     "github.com/luci/luci-go/server/analytics"
//     ...
//   )
//
//   func myHandler(c context.Context, rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
//     ...
//     return templates.Args{
//		   "Analytics": analytics.Snippet(c),
//     }
//   }
//
// And in the base html:
//
// {{ .Analytics }}
package analytics
