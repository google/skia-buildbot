// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package assertions is designed to be a collection of `.` importable, goconvey
// compatible testing assertions, in the style of
// "github.com/smartystreets/assertions".
//
// Due to a bug/feature in goconvey[1], files in this package end in
// `_tests.go`.
//
// This is a signal to goconvey's internal stack traversal logic (used to print
// helpful assertion messages) that the assertions in this package should be
// considered 'testing code' and not 'tested code', and so should be skipped
// over when searching for the first stack frame containing the code under test.
//
// [1]: https://github.com/smartystreets/goconvey/pull/316
package assertions
