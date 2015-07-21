/**
 * Utility javascript functions used across the different CT FE pages.
 *
 */

/**
 * Converts the timestamp used in CTFE DB into a user friendly string.
 **/
function getFormattedTimestamp(timestamp) {
  if (timestamp == 0) {
    return "<pending>";
  }
  var pattern = /(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/;
  return new Date(String(timestamp).replace(pattern,'$1-$2-$3T$4:$5:$6')).toLocaleString();
}

/**
 * Functions to work with information about Chromium builds.
 */
this.chromiumBuild = this.chromiumBuild || function() {
  var chromiumBuild = {};

  /**
   * Returns a Promise that resolves to an array of completed builds.
   **/
  chromiumBuild.getBuilds = function() {
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
  chromiumBuild.getKey = function(build) {
    return build.ChromiumRev + "-" + build.SkiaRev;
  }

  /**
   * Returns a more human-readable GIT commit hash.
   */
  chromiumBuild.shortHash = function(commitHash) {
    return commitHash.substr(0, 7);
  }

  /**
   * Returns a short description for the given build.
   **/
  chromiumBuild.getDescription = function(build) {
    return chromiumBuild.shortHash(build.ChromiumRev) + "-" +
        chromiumBuild.shortHash(build.SkiaRev) + " (Chromium rev created on " +
        getFormattedTimestamp(build.ChromiumRevTs.Int64) + ")";
  }

  return chromiumBuild;
}();
