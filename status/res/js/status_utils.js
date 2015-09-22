/**
 * status_utils.js contains shared utilities used by Skia Status.
 */
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
