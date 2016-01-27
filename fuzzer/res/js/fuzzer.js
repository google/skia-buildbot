/**
 * Utility javascript functions used across the different Fuzzer pages.
 */
this.fuzzer = this.fuzzer || function() {
  "use strict";

  var fuzzer = {};

  fuzzer.getLinkToDetails = function(detailsBase, attr, value) {
    if (!detailsBase || !value) {
      return "#";
    }
    // The file names have "/" in them and the functions can have "(*&" in them.
    // We base64 encode them to prevent problems.
    return detailsBase + "/"+attr+"/" + btoa(value);
  }

  fuzzer.paramFromPath = function(attr) {
    var path = window.location.pathname;
    var start = path.indexOf(attr+"/");
    if (start == -1) {
      return "";
    }
    path = path.slice(start + attr.length + 1);
    var end = path.indexOf("/");
    if (end == -1) {
      return path;
    }
    return path.slice(0, end);
  }

  return fuzzer;
}();
