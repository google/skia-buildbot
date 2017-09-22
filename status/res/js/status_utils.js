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

// Note: The unobfuscate-status extension relies on the existence of the below
//       class names.
var CLASS_BUILD_SINGLE = "build_single";
var CLASS_BUILD_TOP = "build_top";
var CLASS_BUILD_MIDDLE = "build_middle";
var CLASS_BUILD_BOTTOM = "build_bottom";
var CLASS_DASHED_TOP = "dashed_top";
var CLASS_DASHED_BOTTOM = "dashed_bottom";

this.status_utils = this.status_utils || function() {
  "use strict";

  var status_utils = {};
  return status_utils;
}();
