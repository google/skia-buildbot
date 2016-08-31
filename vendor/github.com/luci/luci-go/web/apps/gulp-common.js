/*
  Copyright 2016 The LUCI Authors. All rights reserved.
  Use of this source code is governed under the Apache License, Version 2.0
  that can be found in the LICENSE file.

  This document has been largely derived from the Polymer Starter Kit:
  https://github.com/PolymerElements/polymer-starter-kit
*/

'use strict';

var path = require('path');
var argv = require('yargs').argv;

var exports = module.exports = {}
exports.base = path.join(__dirname, '..');
exports.out = (argv.out || exports.base);
exports.plugins = require('gulp-load-plugins')({
  config: path.join(exports.base, 'package.json'),
});

// Include Gulp & tools we'll use
var $ = exports.plugins;
var del = require('del');
var requireDir = require('require-dir');
var runSequence = require('run-sequence');
var browserSync = require('browser-sync');
var reload = browserSync.reload;
var merge = require('merge-stream');
var fs = require('fs');
var glob = require('glob-all');
var historyApiFallback = require('connect-history-api-fallback');
var crypto = require('crypto');

var AUTOPREFIXER_BROWSERS = [
  'ie >= 10',
  'ie_mob >= 10',
  'ff >= 30',
  'chrome >= 34',
  'safari >= 7',
  'opera >= 23',
  'ios >= 7',
  'android >= 4.4',
  'bb >= 10'
];

exports.setup = function(gulp, config) {
  var APP = path.basename(config.dir);
  var DIST = path.join(exports.out, 'dist', APP);

  var layout = {
    app: APP,
    distPath: DIST,

    // NOTE: Takes vararg via "arguments".
    dist: function() {
      return extendPath(DIST).apply(null, arguments);
    },
  };

  var extendPath = function() {
    var base = [].slice.call(arguments);
    return function() {
      // Simple case: only base, no additional elements.
      if (base.length === 1 && arguments.length === 0) {
        return base[0];
      }

      var parts = base.concat();
      parts.push.apply(parts, arguments)
      return path.join.apply(null, parts);
    };
  };

  var styleTask = function(stylesPath, srcs) {
    return gulp.src(srcs.map(function(src) {
        return path.join(stylesPath, src);
      }))
      .pipe($.autoprefixer(AUTOPREFIXER_BROWSERS))
      .pipe(gulp.dest('.tmp/' + stylesPath))
      .pipe($.minifyCss())
      .pipe(gulp.dest(layout.dist(stylesPath)))
      .pipe($.size({title: stylesPath}));
  };

  var imageOptimizeTask = function(src, dest) {
    return gulp.src(src)
      .pipe($.imagemin({
        progressive: true,
        interlaced: true
      }))
      .pipe(gulp.dest(dest))
      .pipe($.size({title: 'images'}));
  };

  var optimizeHtmlTask = function(src, dest) {
    var assets = $.useref.assets({
      searchPath: ['.tmp', 'app']
    });

    return gulp.src(src)
      .pipe(assets)
      // Concatenate and minify JavaScript
      .pipe($.if('*.js', $.uglify({
        preserveComments: 'some'
      })))
      // Concatenate and minify styles
      // In case you are still using useref build blocks
      .pipe($.if('*.css', $.minifyCss()))
      .pipe(assets.restore())
      .pipe($.useref())
      // Minify any HTML
      .pipe($.if('*.html', $.minifyHtml({
        quotes: true,
        empty: true,
        spare: true
      })))
      // Output files
      .pipe(gulp.dest(dest))
      .pipe($.size({
        title: 'html'
      }));
  };

  // Compile and automatically prefix stylesheets
  gulp.task('styles', function() {
    return styleTask('styles', ['**/*.css']);
  });

  gulp.task('elements', function() {
    return styleTask('elements', ['**/*.css']);
  });

  // Optimize images
  gulp.task('images', function() {
    return imageOptimizeTask('images/**/*', layout.dist('images'));
  });

  // Copy all files at the root level (app)
  gulp.task('copy', function() {
    // Application files.
    var app = gulp.src([
      '*',
      '!inc',
      '!test',
      '!elements',
      '!bower_components',
      '!cache-config.json',
      '!**/.DS_Store',
      '!gulpfile.js',
      '!package.json',
    ]).pipe(gulp.dest(layout.dist()));

    // Copy over only the bower_components we need
    // These are things which cannot be vulcanized
    var webcomponentsjs = gulp.src([
      'inc/bower_components/webcomponentsjs/webcomponents-lite.min.js',
    ]).pipe(gulp.dest(layout.dist('inc/bower_components/webcomponentsjs/')));

    var includes = (config.includes) ? (config.includes(gulp, layout)) : ([]);
    return merge(app, includes, webcomponentsjs)
      .pipe($.size({
        title: 'copy'
      }));
  });

  // Copy web fonts to dist
  gulp.task('fonts', function() {
    return gulp.src(['fonts/**'])
      .pipe(gulp.dest(layout.dist('fonts')))
      .pipe($.size({
        title: 'fonts'
      }));
  });

  // Scan your HTML for assets & optimize them
  gulp.task('html', function() {
    return optimizeHtmlTask(
      ['**/*.html', '!{elements,test,inc}/**/*.html'],
      layout.dist());
  });

  // Vulcanize granular configuration
  gulp.task('vulcanize', function() {
    return gulp.src('elements/elements.html')
      .pipe($.vulcanize({
        stripComments: true,
        inlineCss: true,
        inlineScripts: true
      }))
      .pipe(gulp.dest(layout.dist('elements')))
      .pipe($.size({title: 'vulcanize'}));
  });

  // Clean output directory
  gulp.task('clean', function() {
    var dist = layout.dist();
    var remove = ['.tmp', path.join(dist, '*')];
    var keep = '!'+path.join(dist, '.keep');
    return del(remove.concat(keep), {force: true, dot:true});
  });

  // Watch files for changes & reload
  gulp.task('serve', ['styles', 'elements'], function() {
    browserSync({
      port: 5000,
      notify: false,
      logPrefix: 'PSK',
      snippetOptions: {
        rule: {
          match: '<span id="browser-sync-binding"></span>',
          fn: function(snippet) {
            return snippet;
          }
        }
      },
      // Run as an https by uncommenting 'https: true'
      // Note: this uses an unsigned certificate which on first access
      //       will present a certificate warning in the browser.
      // https: true,
      server: {
        baseDir: ['.tmp', 'app'],
        middleware: [historyApiFallback()]
      }
    });

    gulp.watch(['**/*.html'], reload);
    gulp.watch(['styles/**/*.css'], ['styles', reload]);
    gulp.watch(['elements/**/*.css'], ['elements', reload]);
    gulp.watch(['images/**/*'], reload);
  });

  // Build and serve the output from the dist build
  gulp.task('serve:dist', ['default'], function() {
    browserSync({
      port: 5001,
      notify: false,
      logPrefix: 'PSK',
      snippetOptions: {
        rule: {
          match: '<span id="browser-sync-binding"></span>',
          fn: function(snippet) {
            return snippet;
          }
        }
      },
      // Run as an https by uncommenting 'https: true'
      // Note: this uses an unsigned certificate which on first access
      //       will present a certificate warning in the browser.
      // https: true,
      server: layout.dist(),
      middleware: [historyApiFallback()]
    });
  });

  // Build production files, the default task
  gulp.task('default', ['clean'], function(cb) {
    runSequence(
      ['copy', 'styles', 'images', 'fonts', 'html'],
      'vulcanize',
      cb);
  });
};

require('es6-promise').polyfill();

// Load custom tasks from the `tasks` directory
try {
  require('require-dir')('tasks');
} catch (err) {}
