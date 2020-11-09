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
    return ctfe.getTimestamp(timestamp).toLocaleString();
  }

  /**
   * Converts the timestamp used in CTFE DB into a Javascript timestamp.
   */
  ctfe.getTimestamp = function(timestamp) {
    if (timestamp == 0) {
      return timestamp;
    }
    var pattern = /(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/;
    return new Date(String(timestamp).replace(pattern,'$1-$2-$3T$4:$5:$6'));
  }

  /**
   * Convert from Javascript Date to timestamp recognized by CTFE DB.
   */
  ctfe.getCtDbTimestamp = function(d) {
    var timestamp = String(d.getUTCFullYear()) + sk.human.pad(d.getUTCMonth()+1, 2) +
                    sk.human.pad(d.getUTCDate(), 2) + sk.human.pad(d.getUTCHours(), 2) +
                    sk.human.pad(d.getUTCMinutes(), 2) + sk.human.pad(d.getUTCSeconds(), 2);
    return timestamp
  }

  /**
   * Get user friendly string for repeat after days.
   */
  ctfe.formatRepeatAfterDays = function(num) {
    if (num == 0) {
      return "N/A";
    } else if (num == 1) {
      return "Daily";
    } else if (num == 7) {
      return "Weekly";
    } else {
      return "Every " + num + " days";
    }
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
   * Returns true and displays an error if user has more than 3 active tasks.
   */
  ctfe.moreThanThreeActiveTasks = function(sizeOfUserQueue) {
    if (sizeOfUserQueue > 3) {
        sk.errorMessage("You have " + sizeOfUserQueue + " currently running tasks. Please wait " +
                        "for them to complete before scheduling more CT tasks.");
    }
    return sizeOfUserQueue > 3;
  }

  ctfe.missingLiveSitesWithCustomWebpages = function(customWebpages, benchmarkArgs) {
    if (customWebpages && !benchmarkArgs.includes("--use-live-sites")) {
      sk.errorMessage("Please specify --use-live-sites in benchmark arguments " +
                      "when using custom web pages.");
      return true;
    }
    return false;
  }

  ctfe.getPlaceholderTextForCustomWebpages = function() {
    return "Eg: webpage1,webpage2,webpage3\n\nCommas in webpages should be URL encoded";
  }

  /**
   * Returns a link to the specified google storage path.
   */
  ctfe.getGSLink = function(gsPath) {
    return "https://ct.skia.org/results/cluster-telemetry/" + gsPath;
  }

  /**
   * Returns true if gsPath is not set or if the patch's SHA1 digest in the specified
   * google storage path is for an empty string.
   */
  ctfe.isEmptyPatch = function(gsPath) {
    // Compare against empty string and against the SHA1 digest of an empty string.
    return gsPath === "" || gsPath === "patches/da39a3ee5e6b4b0d3255bfef95601890afd80709.patch";
  }

  /**
   * Returns a string that describes the specified CLs.
   **/
  ctfe.getDescriptionOfCls = function(chromiumClDesc, skiaClDesc, v8ClDesc, catapultClDesc, chromiumBaseBuildClDesc) {
    if (!chromiumClDesc && !skiaClDesc && !v8ClDesc && !catapultClDesc && !chromiumBaseBuildClDesc) {
      return "";
    }
    var str = "Testing ";
    var prev = false;
    if (chromiumClDesc) {
      str += chromiumClDesc;
      prev = true;
    }
    if (skiaClDesc) {
      if (prev) {
        str += " and ";
      }
      str += skiaClDesc;
      prev = true;
    }
    if (v8ClDesc) {
      if (prev) {
        str += " and ";
      }
      str += v8ClDesc;
      prev = true;
    }
    if (catapultClDesc) {
      if (prev) {
        str += " and ";
      }
      str += catapultClDesc;
      prev = true;
    }
    if (chromiumBaseBuildClDesc) {
      if (prev) {
        str += " and ";
      }
      str += chromiumBaseBuildClDesc;
      prev = true;
    }
    return str;
  }

  return ctfe;
}();
