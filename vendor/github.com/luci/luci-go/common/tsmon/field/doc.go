// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package field contains constructors for metric field definitions.
//
// When you create a metric you must specify the names and types of the fields
// that can appear on that metric.  When you increment the metric's value you
// must then pass values for all the fields you defined earlier.
package field
