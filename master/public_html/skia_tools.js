/**
 * @license Copyright 2012 Google Inc.
 *
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file.
 */

/**
 * @fileoverview Tools used by the Skia buildbot dashboards, including
 * facilities for retrieving data from the buildbot master.
 */

"use strict";

var skiaTools = {

globalVariables: null,
googlesourceURL: "https://skia.googlesource.com",

masterHostSuffix: "_master_host",
masterPortSuffix: "_external_port",

/**
 * Retrieve the Git log for the Skia repository.
 *
 * @param {string} startHash
 * @param {string} endHash
 */
gitLog: function(startHash, endHash) {
  var url = this.googlesourceURL + "/skia/+log/" + startHash + ".." + endHash + "?format=JSON";
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", url, false);
  request.send();

  // Remove the first line, which is garbage.
  var responseLines = request.responseText.split('\n');
  responseLines.splice(0, 1);

  return JSON.parse(responseLines.join('\n'))["log"];
},

/**
 * Object used for managing Git history.
 */
GitHistory: function() {
  this.gotRev = {};
  this.allRevisions = [];
  this.lastFetchedRev = "HEAD";

  /**
   * loadCommits
   *
   * @param oldestCommit {string} Load this commit and all newer commits.
   */
  this.loadCommits = function(oldestCommit) {
    var newCommits = skiaTools.gitLog(oldestCommit + "~1", this.lastFetchedRev);
    for (var i = 0; i < newCommits.length; ++i) {
      var commitHash = newCommits[i].commit;
      this.allRevisions.push(commitHash);
      this.gotRev[commitHash] = true;
    }
    this.lastFetchedRev = newCommits[newCommits.length - 1].commit;
  }

  /**
   * getRevList
   *
   * @return {array.<string>} Array of commit hashes in chronological order.
   */
  this.getRevList = function() {
    var revList = Array.prototype.slice.call(this.allRevisions);
    revList.reverse(); // Sort newest to oldest.
    return revList;
  }

  /**
   * ensureLoaded
   *
   * @param commit {string} If this commit has not yet been loaded, load this
   *     and all newer commits.
   */
  this.ensureLoaded = function(commit) {
    if (!this.gotRev[commit]) {
      this.loadCommits(commit);
    }
  }
},

/**
 * Convenience function for populating a ComboBox or ListBox with a list of
 * items. Existing items will be cleared.
 *
 * @param {string} menuId ID of the menu to populate.
 * @param {Array.<string>} items A list of strings to insert into the menu.
 */
populateMenu: function(menuId, items) {
  var menu = document.getElementById(menuId);
  menu.options.length = 0;
  for (var itemIdx = 0; itemIdx < items.length; itemIdx++) {
    var item = items[itemIdx];
    var newOption = document.createElement("option");
    newOption.text = item;
    newOption.value = item;
    menu.options.add(newOption);
  }
},

/**
 * Load the global_variables.json file.
 */
loadGlobalVariables: function() {
  var url = this.googlesourceURL + "/buildbot/+/master/site_config/" +
      "global_variables.json?format=TEXT";
  try {
    var request = new XMLHttpRequest();
  } catch (error) {
    alert(error);
  }
  request.open("GET", url, false);
  request.send();
  this.globalVariables = JSON.parse(atob(request.responseText));
},

/**
 * Retrieve the given variable.
 *
 * @return The value for the requested variable, as defined in the global
 *     variables file, or undefined if it is not found.
 */
getVariable: function(varName) {
  if (!this.globalVariables) {
    this.loadGlobalVariables();
  }
  if (!this.globalVariables[varName]) {
    return undefined;
  }
  return this.globalVariables[varName].value;
},

/**
 * Information about a single build.
 */
Build: function(builder, number, revision, result, startTime, endTime, steps) {
  this.builder     = builder;
  this.number      = number;
  this.revision    = revision;
  this.result      = result;
  this.startTime   = startTime;
  this.endTime     = endTime;
  this.elapsedTime = endTime - startTime;
  this.steps       = steps;

  /**
   * getBuilder
   *
   * @return {string} The name of the builder who owns this build.
   */
  this.getBuilder     = function() { return this.builder; }

  /**
   * getNumber
   *
   * @return {number} The build number.
   */
  this.getNumber      = function() { return this.number; }

  /**
   * getResult
   *
   * @return {number} The result of the build. Will be 0 iff the build
   *     succeeded.
   */
  this.getResult      = function() { return this.result; }

  /**
   * getRevision
   *
   * @return {number} The revision number of this build.
   */
  this.getRevision    = function() { return this.revision; }

  /**
   * getStartTime
   *
   * @return {number} Start time of the build in UNIX seconds from epoch.
   */
  this.getStartTime   = function() { return this.startTime; }

  /**
   * getEndTime
   *
   * @return {number} End time of the build in UNIX seconds from epoch.
   */
  this.getEndTime     = function() { return this.endTime; }

  /**
   * getElapsedTime
   *
   * @return {number} Elapsed time of the build in seconds.
   */
  this.getElapsedTime = function() { return this.elapsedTime; }

  /**
   * getStartTime
   *
   * @return {Array.<BuildStep>} Information about the steps of this build.
   */
  this.getSteps       = function() { return this.steps; }
},

/**
 * Information about a single build step.
 */
BuildStep: function(name, elapsedTime, result, stdio) {
  this.name        = name;
  this.elapsedTime = elapsedTime;
  this.result      = result;
  this.stdio       = stdio;

  /**
   * getName
   *
   * @return {string} The name of the build step.
   */
  this.getName        = function() { return this.name; }

  /**
   * getElapsedTime
   *
   * @return {number} Elapsed time of the build step in seconds.
   */
  this.getElapsedTime = function() { return this.elapsedTime; }

  /**
   * getResult
   *
   * @return {number} The result of the build. Will be 0 iff the build step
   *     succeeded.
   */
  this.getResult      = function() { return this.result; }

  /**
   * getStdio
   *
   * @return {string} URL to the log output for this build step.
   */
  this.getStdio       = function() { return this.stdio; }
},

/**
 * Information about a builder.
 */
Builder: function(name, master, basedir, cachedBuilds, category, currentBuilds,
                  slaves, state) {
  this.name                = name;
  this.master              = master;
  this.basedir             = basedir;
  // alreadyLoadedBuilds differs from the passed-in cachedBuilds; the passed-in
  // parameter refers to the builds which the build master has cached. This
  // property is a set of builds which have been loaded from the build master
  // and stored here for future use.
  this.alreadyLoadedBuilds = {};
  this.category            = category;
  this.currentBuilds       = currentBuilds;
  this.lastBuild           = cachedBuilds[cachedBuilds.length - 1];
  this.slaves              = slaves;
  this.state               = state;

  /**
   * getName
   *
   * @return {string} The name of the builder.
   */
  this.getName          = function() { return this.name; }

  /**
   * getBaseDir
   *
   * @return {string} Directory on the build slave machine in which build
   *     information is stored. This is typically the same as the builder name.
   */
  this.getBaseDir       = function() { return this.basedir; }

  /**
   * getCategory
   *
   * @return {string} Category of this builder. This is the heading under
   *     which the builder is placed on the buildbot web page.
   */
  this.getCategory      = function() { return this.category; }

  /**
   * getCurrentBuilds
   *
   * @return {Array.<number>} List of currently-running builds for this
   *     builder.
   */
  this.getCurrentBuilds = function() { return this.currentBuilds; }

  /**
   * getLastBuild
   *
   * @return {number} The build number of the last completed build for this
   *     builder.
   */
  this.getLastBuild = function() { return this.lastBuild; }

  /**
   * getSlaves
   *
   * @return {Array.<string>} List of known build slaves which are capable of
   *     running builds for this builder.
   */
  this.getSlaves        = function() { return this.slaves; }

  /**
   * getState
   *
   * @return {string} Current status of the builder. Either "building" or
   *     "idle."
   */
  this.getState         = function() { return this.state; }

  /**
   * Obtain information about a single build from the buildbot master. This is
   * synchronous and should be assumed to be very slow.
   *
   * @param {number} build The number of the build which should be retrieved.
   * @param {boolean} loadUnfinished Whether or not to load data for unfinished
   *     builds.
   * @param {boolean} loadUnknownRevs Whether or not to load data for builds
   *     which do not have an associated revision number. This occurs when the
   *     source checkout step fails.
   *
   * @return {object|null} A Build instance containing information about the
   *     requested build.
   */
  this.loadDataForBuild = function(build, loadUnfinished, loadUnknownRevs) {
    var buildData = this.master.loadData("builders/" + this.getName() +
        "/builds/" + build + "/steps");

    // Build step results.
    var SUCCESS = 0;
    var FAILURE = 2;
    var SKIPPED = 3;

    var steps = [];
    var result = 0;
    var startTime = 0;
    var endTime = 0;
    var revision = undefined;
    var gotRevisionStr = "got_revision: ";
    for (var step in buildData) {
      var stepData = buildData[step];
      if (stepData["isStarted"] && !stepData["isFinished"] &&
          !loadUnfinished) {
        // If the build isn't finished, ignore it
        return null;
      }
      if (!stepData["isStarted"]) {
        continue;
      }
      if (stepData["name"] == "Update") {
        // The buildbot's JSON interface stores results as an array in which
        // the first element is an integer indicating success or failure.
        if (stepData["isStarted"] && stepData["isFinished"] &&
            stepData["results"][0] == 0) {
          // The "text" field is an array containing extra information about
          // the build step. In the case of the Update step, its second element
          // is a string indicating the revision obtained for the current
          // build.
          revision = stepData["text"][1].substring(gotRevisionStr.length);
        } else if (!loadUnknownRevs) {
          // If the Update step failed, we can't attach a revision, so we have
          // to ignore this build.
          console.log("Warning: Can't get a revision for build #" + build +
                      ". Skipping.");
          return null;
        }
      }
      var times = stepData["times"];
      var stepTime = times[1] - times[0];
      if (startTime == 0) {
        startTime = times[0];
      }
      endTime = times[1];
      var stdout = null;
      try {
        stdout = stepData["logs"][0][1];
      } catch(e) {
        stdout = "None";
      }

      var buildStep = new skiaTools.BuildStep(stepData["name"], stepTime,
                                              stepData["results"][0], stdout);
      steps.push(buildStep);

      if (buildStep.getResult() != SUCCESS &&
          buildStep.getResult() != SKIPPED) {
        result = FAILURE;
      }
    }
    if (revision == undefined) {
      console.log("Warning: could not find a revision for build #" + build);
    }

    return new skiaTools.Build(this.getName(), build, revision, result,
                               startTime, endTime, steps);
  }

  /**
   * Obtain information about the builds for a single builder. Works backward
   * from the last known build, loading builds until the requested number of
   * builds has been fulfilled or all of the builder's builds have been loaded.
   * This is synchronous and should be assumed to be very slow.
   *
   * @param {number} numRevs The number of revisions to load.
   *
   * @return {object} Instances of Build for the builder, indexed by revision.
   */
  this.loadBuilds = function(numRevs) {
    var data = {};
    var lastBuild = this.getLastBuild();
    for (var buildNum = lastBuild; buildNum >= lastBuild - numRevs && buildNum >= 0; buildNum--) {
      var build = this.alreadyLoadedBuilds[buildNum];
      if (!build) {
        build = this.loadDataForBuild(buildNum, false, false);
        if (!build) { continue; }
        this.alreadyLoadedBuilds[buildNum] = build;
      }
      data[buildNum] = build;
    }
    return data;
  }
},

/**
 * Information about a build slave.
 */
BuildSlave: function(admin, builders, connected, currentBuilds, host, name,
                     master, version) {
  this.admin         = admin;
  this.builders      = builders;
  this.connected     = connected;
  this.currentBuilds = currentBuilds;
  this.host          = host;
  this.name          = name;
  this.master        = master;
  this.version       = version;

  /**
   * getAdmin
   *
   * @return {string} Usernames of buildbot maintainers.
   */
  this.getAdmin         = function() { return this.admin; }

  /**
   * getBuilders
   *
   * @return {object} Dictionary whose keys are builder names and values are
   *     lists of build numbers indicating which builds for which builders this
   *     slave has performed.
   */
  this.getBuilders      = function() { return this.builders; }

  /**
   * isConnected
   *
   * @return {boolean} Whether or not the build slave is currently connected
   *     to the build master.
   */
  this.isConnected      = function() { return this.connected; }

  /**
   * getCurrentBuilds
   *
   * @return {Array.<object>} List of dictionaries containing information about
   *     currently-running builds on this slave.
   */
  this.getCurrentBuilds = function() { return this.currentBuilds; }

  /**
   * getHost
   *
   * @return {string} Hostname of this build slave.
   */
  this.getHost          = function() { return this.host; }

  /**
   * getName
   *
   * @return {string} The name of this build slave.
   */
  this.getName          = function() { return this.name; }

  /**
   * getVersion
   *
   * @return {string} Version of BuildBot which this build slave is running.
   */
  this.getVersion       = function() { return this.version; }

  /**
   * Obtain information about recent builds for a build slave. This is
   * synchronous and should be assumed to be very slow.
   *
   * @param {number} rangeMin Builds before this time will not be loaded.
   * @param {number} currentTime The time at which {@code slaveDict} was obtained
   *     from the build master. This value is passed in rather than obtaining the
   *     current time at the call of this function in case the state of the build
   *     slave has changed since {@code slaveDict} was obtained.
   *
   * @return {Array.<Build>} List of Build objects.
   */
  this.loadBuilds = function(rangeMin, currentTime) {
    var builders = this.getBuilders();
    var buildList = [];
    for (var builder in builders) {
      var builderObj = new skiaTools.Builder(builder, this.master);
      var builds = builders[builder];
      for (var buildIdx = 0; buildIdx < builds.length; buildIdx++) {
        var buildNum = builds[buildIdx];
        var build = builderObj.loadDataForBuild(buildNum, true, true);
        if (build) {
          buildList.push(build);
          if (build.getEndTime() < rangeMin || build.getStartTime() < rangeMin) {
            break;
          }
        }
      }
    }
    var runningBuilds = this.getCurrentBuilds();
    for (var buildIdx = 0; buildIdx < runningBuilds.length; buildIdx++) {
      var buildData = runningBuilds[buildIdx];
      buildList.push(new this.Build(buildData["builderName"],
                                    buildData["number"],
                                    -1, 0, buildData["times"][0],
                                    currentTime + 1, []));
    }
    buildList.sort(function(a, b) {
      return a.getStartTime() - b.getStartTime();
    });
    return buildList;
  }
},

loadMasterList: function() {
  var masters = [];
  if (!this.globalVariables) {
    this.loadGlobalVariables();
  }
  for (var key in this.globalVariables) {
    var suffixIndex = key.indexOf(
        skiaTools.masterHostSuffix,
        key.length - skiaTools.masterHostSuffix.length);
    if (suffixIndex !== -1) {
      masters.push(key.substring(0, suffixIndex));
    }
  }
  return masters;
},

Master: function(name) {
  this.name = name;
  this.host = skiaTools.getVariable(name + skiaTools.masterHostSuffix);
  this.port = skiaTools.getVariable(name + skiaTools.masterPortSuffix);

  /**
   * getName
   *
   * @return {string} Name of this build master.
   */
  this.getName          = function() { return this.name; }

  /**
   * getHost
   *
   * @return {string} Hostname of this build master.
   */
  this.getHost          = function() { return this.host; }

  /**
   * getPort
   *
   * @return {string} Port number of this build master.
   */
  this.getPort          = function() { return this.port; }

  /**
   * loadData
   *
   * Sends an {@code XMLHttpRequest} to the buildbot master, parses the JSON in
   * the response, and returns a dictionary. This is synchronous and should be
   * assumed to be very slow.
   *
   * @param {string} subdir Subdirectory of the buildbot master's JSON
   *     interface to query.
   */
  this.loadData         = function(subdir) {
    try {
      var request = new XMLHttpRequest();
    } catch (error) {
      alert(error);
    }
    var url = "http://" + this.getHost() + ":" + this.getPort() + "/json/" +
        subdir;
    request.open("GET", url, false);
    request.send(null);
    return JSON.parse(request.responseText);
  }

  /**
   * Obtain high-level information about known builders. This is synchronous and
   * should be assumed to be very slow.
   *
   * @return {Array.<Builder>} A list of Builder objects.
   */
  this.loadBuilders = function() {
    var builders = [];
    var buildersData = this.loadData("builders");
    for (var builderName in buildersData) {
      var builderData = buildersData[builderName];
      builders.push(new skiaTools.Builder(builderName, this,
                                          builderData["basedir"],
                                          builderData["cachedBuilds"],
                                          builderData["category"],
                                          builderData["currentBuilds"],
                                          builderData["slaves"],
                                          builderData["state"]));
    }
    return builders;
  }

  /**
   * Obtain high-level information about known build slaves. This is synchronous
   * and should be assumed to be very slow.
   *
   * @return {Array.<BuildSlave>} A list of BuildSlave objects.
   */
  this.loadSlaves = function(host, port) {
    var slaves = [];
    var slavesData = this.loadData("slaves");
    for (var slave in slavesData) {
      var slaveData = slavesData[slave];
      var currentBuilds = slaveData["currentBuilds"];
      if (currentBuilds == undefined) {
        currentBuilds = [];
      }
      for (var buildIdx = 0; buildIdx < currentBuilds.length; buildIdx++) {

      }
      slaves.push(new skiaTools.BuildSlave(slaveData["admin"],
                                           slaveData["builders"],
                                           slaveData["connected"],
                                           currentBuilds,
                                           slaveData["host"],
                                           slaveData["name"],
                                           this,
                                           slaveData["version"]));
    }
    return slaves;
  }

},

};
