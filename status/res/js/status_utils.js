/**
 * status_utils.js contains shared utilities used by Skia Status.
 */

var BUILDBOT_PENDING = 0;

var BUILDBOT_RESULT_SUCCESS = 0;
var BUILDBOT_RESULT_WARNINGS = 1;
var BUILDBOT_RESULT_FAILURE = 2;
var BUILDBOT_RESULT_SKIPPED = 3;
var BUILDBOT_RESULT_EXCEPTION = 4;
var BUILDBOT_RESULT_CANCELLED = 5;

var CLASS_BUILD_SINGLE = "build_single";
var CLASS_BUILD_TOP = "build_top";
var CLASS_BUILD_MIDDLE = "build_middle";
var CLASS_BUILD_BOTTOM = "build_bottom";
var CLASS_DASHED_TOP = "dashed_top";
var CLASS_DASHED_BOTTOM = "dashed_bottom";

this.status_utils = this.status_utils || function() {
  "use strict";

  var status_utils = {};


  // Global variables.
  status_utils.buildbotUrlPrefixInternal = "https://uberchromegw.corp.google.com/i";
  status_utils.buildbotUrlPrefixExternal = "http://build.chromium.org/p";

  /*
   * getBuildbotUrlPrefix determines the buildbot URL prefix for the given build.
   */
  status_utils.getBuildbotUrlPrefix = function(build, internalView) {
    var buildbotUrlPrefix = status_utils.buildbotUrlPrefixExternal + build.master;
    for (var i = 0; i < build.properties.length; i++) {
      if (build.properties[i][0] == "buildbotURL") {
        buildbotUrlPrefix = build.properties[i][1];
        break;
      }
    }
    if (internalView) {
      buildbotUrlPrefix = buildbotUrlPrefix.replace(status_utils.buildbotUrlPrefixExternal,
                                                    status_utils.buildbotUrlPrefixInternal);
    }
    return buildbotUrlPrefix;
  };

  return status_utils;
}();
