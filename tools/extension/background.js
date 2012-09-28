// Copyright (c) 2012 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @fileoverview Code related to the background page. Makes requests to the 
 * different builders and keeps track of the bot statuses.
 */

var BASE_URL = 'http://70.32.156.53:10117/';
var STATUS_URL = BASE_URL + 'builders';
var INITIAL_CORE_BUILDER_PATTERNS = ['^Skia.*Shuttle_', '^Skia.*NexusS_',
  '^Skia.*Xoom_', '^Skia.*GalaxyNexus_', '^Skia.*Nexus7_', '^Skia.*Mac_',
  '^Skia.*MacMiniLion_', '^Skia.*PerCommit_', '^Skia.*Periodic_',
  '^Skia.*Linux_'];
var INITIAL_INTERVAL = 60; // seconds
var MILLIS_PER_SECOND = 1000;

var LS_INTERVAL = 'check_interval_sec';
var LS_PATTERNS = 'core_builder_patterns';

var checkIntervalMillis;
var coreBuilderPatterns;
var botStatusList = [];

/**
 * Writes debug messages to the console.
 * @param {string} msg The message to write to the console.
 */
function debug(msg) {
  console.log(msg);
}

/**
 * Initializes local storage settings for the extension if they aren't set.
 */
function initOptions() {
  localStorage[LS_INTERVAL] = INITIAL_INTERVAL;
  localStorage[LS_PATTERNS] = JSON.stringify(INITIAL_CORE_BUILDER_PATTERNS);
}

/**
 * Loads the saved settings from local storage or initializes them.
 */
function loadOptions() {
  if (!localStorage[LS_INTERVAL] || !localStorage[LS_PATTERNS]) {
    initOptions();
  }

  checkIntervalMillis = parseInt(localStorage[LS_INTERVAL]);
  debug('Loaded check_interval_sec: ' + checkIntervalMillis);
  if (isNaN(checkIntervalMillis)) {
    debug('  ==> Reset to default');
   checkIntervalMillis = INITIAL_INTERVAL;
  }

  try {
    coreBuilderPatterns = JSON.parse(localStorage[LS_PATTERNS]);
  } catch (e) {}
  
  debug('Loaded core_builder_patterns: ' + coreBuilderPatterns);
  if (!(coreBuilderPatterns instanceof Array) || coreBuilderPatterns.length <= 0
    || !(typeof coreBuilderPatterns[0] == "string")) {
    debug('  ==> Reset to default');
    coreBuilderPatterns = INITIAL_CORE_BUILDER_PATTERNS;
  }
}

/**
 * Returns whether a given builder is one of the core builders that the
 * extension is monitoring.
 * @param {string} name The name of the builder to check.
 * @return {boolean} Whether this builder is a core builder.
 */
function isCoreBuilder(name) {
  for (var i = 0; i < coreBuilderPatterns.length; i++) {
    if (name.match(new RegExp(coreBuilderPatterns[i]))) {
      return true;
    }
  }
  return false;
}

function evalString(doc, exp, node) {
  return doc.evaluate(exp, node, null, XPathResult.STRING_TYPE, null)
  .stringValue;
}

function classesToStatus(str) {
  var arr = str.split(' ');
  for (var i = 0; i < arr.length; i++) {
    if (arr[i] == 'success') {
      return true;
    }
  }
  return false;
}

/**
 * Updates the statuses for each buildbot and updates the browser action badge.
 * @param {string} statusDocText The HTML response that should be parsed to get
 *     the bot statuses.
 */
function updateStatus(statusDocText) {
  var d = document;
  var parent = d.getElementById('parent');
  parent.innerHTML = statusDocText;
  var botNodes = d.evaluate('.//tbody[1]/tr', parent, null,
    XPathResult.UNORDERED_NODE_ITERATOR_TYPE, null);
  var statusList = [];
  var botNode;
  while (botNode = botNodes.iterateNext()) {
    var botStatus = {};
    botStatus.name = evalString(d, 'td[1]', botNode);
    botStatus.success = classesToStatus(evalString(d, 'td[2]/@class', botNode));
    botStatus.statusText = evalString(d, 'td[2]', botNode);
    botStatus.statusLink = evalString(d, 'td[2]/a[1]/@href', botNode);
    statusList.push(botStatus);
  }

  botStatusList = statusList;
  updateBadge();
}

/**
 * Updates the browser action badge depending on whether the core builders are
 * failing or succeeding. If all the core builders are succeeding, then it will
 * be green, otherwise it will be red.
 */
function updateBadge() {
  var coreGreen = true;
  for (var i = 0; i < botStatusList.length; i++) {
    if (isCoreBuilder(botStatusList[i].name) && !botStatusList[i].success) {
      coreGreen = false;
      break;
    }
  }

  if (coreGreen) {
    chrome.browserAction.setBadgeText({text:"\u2022"});
    chrome.browserAction.setBadgeBackgroundColor({color:[0,255,0,255]});
  } else {
    chrome.browserAction.setBadgeText({text:"\u00D7"});
    chrome.browserAction.setBadgeBackgroundColor({color:[255,0,0,255]});
  }
}

/**
 * Makes an XHR request to the specified URL and then calls the callback
 * function with the HTTP response text.
 * @param {string} url The URL for the request.
 * @param {string} callback The name of the function to call.
 */
function requestURL(url, callback) {
  var xhr = new XMLHttpRequest();
  try {
    xhr.onreadystatechange = function(state) {
      if (xhr.readyState == 4) {
        if (xhr.responseText) {
          callback(xhr.responseText);
        }
      }
    }

    xhr.onerror = function(error) {
      debug("xhr error: " + JSON.stringify(error));
    }

    xhr.open("GET", url, true);
    xhr.send({});
  } catch(e) {
    debug("exception: " + e);
  }
}

/**
 * Calls the requestURL function, and then repeatedly calls itself via
 * setTimeout based on the checkIntervalMillis setting, which defaults to every 60
 * seconds.
 */
function requestStatus() {
  requestURL(STATUS_URL, updateStatus);
  setTimeout(requestStatus, checkIntervalMillis * MILLIS_PER_SECOND);
}

// When the page loads, load the options and then kick off the requestStatus.
window.onload = function() {
  loadOptions();
  setTimeout(requestStatus, MILLIS_PER_SECOND);
}


