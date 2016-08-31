/*
  Copyright 2016 The LUCI Authors. All rights reserved.
  Use of this source code is governed under the Apache License, Version 2.0
  that can be found in the LICENSE file.
*/

'use strict';

var packageJsonFile = '../../package.json';

// Include Gulp & tools we'll use
var gulp = require('gulp');
var luci = require('../gulp-common.js');

luci.setup(gulp, {
  dir: __dirname,
});
