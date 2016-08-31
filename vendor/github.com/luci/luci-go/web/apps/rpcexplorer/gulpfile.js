/*
  Copyright 2016 The LUCI Authors. All rights reserved.
  Use of this source code is governed under the Apache License, Version 2.0
  that can be found in the LICENSE file.
*/

'use strict';

// Include Gulp & tools we'll use
var gulp = require('gulp');
var luci = require('../gulp-common.js');
var path = require('path');

luci.setup(gulp, {
  dir: __dirname,
  includes: function(gulp, layout) {
    return gulp.src([
        path.join('inc', 'bower_components', 'bootstrap', 'dist', 'css',
                  'bootstrap.min.css'),
    ]).pipe(gulp.dest(layout.dist('inc/bower_components/bootstrap/dist/css')));
  },
});
