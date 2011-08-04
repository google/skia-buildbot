// Copyright (c) 2011 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Use the JSON functionality to parse a json string, and fallback to
// the eval method is JSON is not supported by the browser.
function parseJSON(data) {
  if (typeof (JSON) !== 'undefined' &&
      typeof (JSON.parse) === 'function') {
    return JSON.parse(data);
  } else {
    return eval('(' + data + ')');
  }
}

// Return the name associated with a buildbot result code.
function getResultName(result) {
  if (result == 0) {
    return 'success';
  } else if (result == 2) {
    return 'failure';
  } else {
    return 'exception';
  }
}

// Parse the JSON of the reliability builder page and return the list of recent
// builds, newest first.
function parseReliability(xmlHttp) {
  if (xmlHttp.readyState == 4 && xmlHttp.status == 200) {
    var reliabilityData = parseJSON(xmlHttp.responseText);
    cachedBuilds = reliabilityData['cachedBuilds'];
    cachedBuilds.sort();
    cachedBuilds.reverse();
    return cachedBuilds;
  }

  return null;
}

// Parse the JSON of a reliability build and return it.
function parseBuild(xmlHttp) {
  if (xmlHttp.readyState == 4 && xmlHttp.status == 200) {
    return parseJSON(xmlHttp.responseText);
  }
  return null;
}

// Parse the stdio from a reliability build and return the crash results.
// NOTE: This function is highly dependent on the format of the text generated
// by the reliability script. It will break if the format changes.
function parseStdio(xmlHttp)  {
  var buildResults = {};
  buildResults.webCrash = null;
  buildResults.webCount = null;
  buildResults.uiCrash = null;
  buildResults.uiCount = null;

  if (xmlHttp.readyState == 4 && xmlHttp.status == 200) {
    lines = xmlHttp.responseText.split('\n');
    for (var i = 0; i < lines.length; i++) {
      var re = new RegExp('\\bsuccess: (\\d+); crashes: (\\d+); ' +
                          'crash dumps: (\\d+); timeout: (\\d+)\\b');
      var match = re.exec(lines[i]);
      if (match != null) {
        if (buildResults.webCrash == null) {
          buildResults.webCrash = match[2];
          buildResults.webCount = match[1];
        } else {
          buildResults.uiCrash = match[2];
          buildResults.uiCount = match[1];
        }
      }
    }
  }
  return buildResults;
}

function displayReliability() {
  var xmlHttp = new XMLHttpRequest();
  var jsonPath = '../chromium/json/builders/Win%20Reliability';
  var buildPath = '../chromium/builders/Win%20Reliability/builds/';
  // NOTE: This will break if the name of the step changes.
  var stepLog = '/steps/reliability%3A%20partial%20result%20of%20current%20' +
                'build/logs/stdio';

  // Get the main reliability page.
  xmlHttp.open('GET', jsonPath, false);
  xmlHttp.send(null);
  var cachedBuilds = parseReliability(xmlHttp);

  // Information we need to get from the builds.
  var lastBuild = null;
  var lastSuccess = null;
  var lastResult = null;

  // Iterate through all the builds to find the last one that is finished and
  // the last green build.
  if (cachedBuilds != null) {
    for (var i = 0; i < cachedBuilds.length; i++) {
      xmlHttp.open('GET', jsonPath + '/builds/' + cachedBuilds[i], false);
      xmlHttp.send(null);
      var currentBuild = parseBuild(xmlHttp);

      if (currentBuild != null) {
        if (lastBuild == null && currentBuild['results'] != null) {
          lastBuild = currentBuild;
          lastResult = getResultName(currentBuild['results']);
        }

        if (currentBuild['results'] == 0) {
          lastSuccess = currentBuild;
        }
      }

      if (lastBuild != null && lastSuccess != null) {
        break;
      }
    }
  }

  // Information we need to get from the last finished build.
  var buildResults = null;
  var lastLog = null;

  // Fetch the crash count.
  if (lastBuild != null) {
    lastLog = buildPath + lastBuild['number'] + stepLog;
    xmlHttp.open('GET', lastLog, false);
    xmlHttp.send(null);
    buildResults = parseStdio(xmlHttp);
  }

  var topBox = document.getElementById('ReliabilityTop');
  var bottomBox = document.getElementById('ReliabilityBottom');

  // Update the top box.
  if (lastBuild == null || buildResults.webCrash == null ||
      buildResults.uiCrash == null) {
    // No information about the current build.
    topBox.className = 'exception';
    topBox.innerHTML = '<p style="margin-bottom: 25px; margin-top: 20px">' +
                       'stats<br>not available</p>';
  } else {
    // TODO(nsylvain): Get full range.
    var rev = lastBuild['sourceStamp']['revision'];
    var revisionRange = rev + ' : ' + rev;
    var webUrl = 'http://chromebot/buildsummary?id=buildbot_' + rev +  '_ext';
    var webCrashStr = buildResults.webCrash + ' / ' +
                      buildResults.webCount + ' URLs';
    var uiUrl = 'http://chromebot/buildsummary?id=ui_buildbot_' + rev;
    var uiCrashStr = buildResults.uiCrash + ' / ' +
                     buildResults.uiCount + ' UI Ops';

    var text = '<p style="margin-bottom: 7px; margin-top: 2px;">' +
               '<a href="' + lastLog + '">' + revisionRange + '</a>' +
               '<br><br>Crashes:<br>' +
               '<a href="' + webUrl + '">' + webCrashStr + '</a><br>' +
               '<a href="' + uiUrl + '">' + uiCrashStr + '</a></p>';

    topBox.className = lastResult;
    topBox.innerHTML = text;
  }

  // Update the bottom box.
  if (lastResult == 'success') {
    var d = new Date();
    var diff_ms = d.getTime() - (lastBuild['times'][1] * 1000);
    var diff_minute = parseInt(diff_ms / 1000 / 60);

    bottomBox.className = 'success';
    bottomBox.innerHTML = diff_minute + ' minutes ago';
  } else if (lastSuccess != null) {
    var rev = lastSuccess['sourceStamp']['revision'];
    var lastGreenLog = buildPath + lastSuccess['number'] + stepLog;
    var text = 'Last green: <a href="' + lastGreenLog + '">' + rev + '</a>';

    bottomBox.className = 'success'
    bottomBox.innerHTML = text;
  } else {
    bottomBox.className = topBox.className;
    bottomBox.innerHTML = '&nbsp;';
  }
}
