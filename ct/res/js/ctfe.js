/**
 * Utility javascript functions used across the different CT FE pages.
 */
this.ctfe = this.ctfe || function() {
  "use strict";

  var ctfe = {};

  /**
   * Converts the timestamp used in CTFE DB into a user friendly string.
   **/
  ctfe.getFormattedTimestamp = function(timestamp) {
    if (timestamp == 0) {
      return "<pending>";
    }
    var pattern = /(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/;
    return new Date(String(timestamp).replace(pattern,'$1-$2-$3T$4:$5:$6')).toLocaleString();
  }

  /**
   * Functions to work with information about page sets.
   */
  ctfe.pageSets = {};

  /**
   * Returns a Promise that resolves to an array of defined page sets.
   **/
  ctfe.pageSets.getPageSets = function() {
    return sk.post("/_/page_sets/")
        .then(JSON.parse);
  }

  /**
   * Returns an identifier for the given page set.
   **/
  ctfe.pageSets.getKey = function(pageSet) {
    return pageSet.key;
  }

  /**
   * Returns a short description for the given page set.
   **/
  ctfe.pageSets.getDescription = function(pageSet) {
    return pageSet.description;
  }

  /**
   * Functions to work with information about Chromium builds.
   */
  ctfe.chromiumBuild = {};

  /**
   * Returns a Promise that resolves to an array of completed builds.
   **/
  ctfe.chromiumBuild.getBuilds = function() {
    var queryParams = {
      "size": 20,
      "successful": true,
    }
    var queryStr = "?" + sk.query.fromObject(queryParams);
    return sk.post("/_/get_chromium_build_tasks" + queryStr)
        .then(JSON.parse)
        .then(function (json) {
          return json.data;
        });
  }

  /**
   * Returns an identifier for the given build.
   **/
  ctfe.chromiumBuild.getKey = function(build) {
    return build.ChromiumRev + "-" + build.SkiaRev;
  }

  /**
   * Returns a more human-readable GIT commit hash.
   */
  ctfe.chromiumBuild.shortHash = function(commitHash) {
    return commitHash.substr(0, 7);
  }

  /**
   * Returns a short description for the given build.
   **/
  ctfe.chromiumBuild.getDescription = function(build) {
    return ctfe.chromiumBuild.shortHash(build.ChromiumRev) + "-" +
        ctfe.chromiumBuild.shortHash(build.SkiaRev) + " (Chromium rev created on " +
        ctfe.getFormattedTimestamp(build.ChromiumRevTs.Int64) + ")";
  }

  return ctfe;
}();
