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
   * Returns a Promise that resolves to an array of builds.
   **/
  chromiumBuild.getBuilds = function() {
    return sk.post('/_/chromium_builds/').then(JSON.parse);
  }

  /**
   * Returns an identifier for the given build.
   **/
  chromiumBuild.getKey = function(chromiumBuild) {
    return chromiumBuild.key;
  }

  /**
   * Returns a short description for the given build.
   **/
  chromiumBuild.getDescription = function(chromiumBuild) {
    return chromiumBuild.description;
  }

  return chromiumBuild;
}();
