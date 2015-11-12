/**
 * Utility javascript functions used across the different Fuzzer pages.
 */
this.fuzzer = this.fuzzer || function() {
  "use strict";

  var fuzzer = {};

  fuzzer.getLinkToDetails = function(detailsBase, fuzzType, attr, value) {
    if (!detailsBase || !value) {
      return "#";
    }
    // String.prototype.endsWith is ECMA 6, so check for it to fail gracefully
    if (!(detailsBase.endsWith && detailsBase.endsWith("?"))) {
      detailsBase = detailsBase + "&";
    }
    var base = detailsBase + attr + "=" + value;
    if (fuzzType === undefined) {
      return base;
    }
    return base + "&fuzz-type=" + fuzzType;
  }

  return fuzzer;
}();
