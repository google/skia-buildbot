// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function() {

  function updateBadgeOnErrorStatus() {
    chrome.browserAction.setBadgeText({text:'?'});
    chrome.browserAction.setBadgeBackgroundColor({color:[0,0,255,255]});
  }

  var lastNotification = null;
  function notifyStatusChange(treeState, status) {
    if (lastNotification)
      lastNotification.close();

    lastNotification = new Notification('Skia Tree is ' + treeState, {
      icon: chrome.extension.getURL('pencil128_' + treeState + '.png'),
      body: status
    });
  }

  function updateTimeBadge(lastChangeTime) {
    chrome.browserAction.setBadgeText(
      {text: sk.human.diffDate(lastChangeTime)});
  }

  // The type parameter should be 'open', 'closed', or 'caution'.
  function getLastStatusTime(callback, type) {
    sk.get('http://skia-tree-status.appspot.com/allstatus?limit=20&format=json').then(function(text) {
      var entries = JSON.parse(text);

      for (var i = 0; i < entries.length; i++) {
        if (entries[i].general_state == type) {
          callback(new Date(entries[i].date + ' UTC'));
          return;
        }
      }
    }).catch(updateBadgeOnErrorStatus);
  }

  var lastState;
  var lastChangeTime;
  function updateStatus(status) {
    var badgeState = {
      open: {color: 'green', defaultText: '\u2022'},
      closed: {color: 'red', defaultText: '\u00D7'},
      caution: {color: '#CDCD00', defaultText: '!'}
    };

    chrome.browserAction.setTitle({title:status});
    var treeState = (/open/i).exec(status) ? 'open' :
        (/caution/i).exec(status) ? 'caution' : 'closed';

    if (lastState && lastState != treeState) {
      notifyStatusChange(treeState, status);
    }

    chrome.browserAction.setBadgeBackgroundColor(
        {color: badgeState[treeState].color});

    if (lastChangeTime === undefined) {
      chrome.browserAction.setBadgeText(
          {text: badgeState[treeState].defaultText});
      lastState = treeState;
      getLastStatusTime(function(time) {
        lastChangeTime = time;
        updateTimeBadge(lastChangeTime);
      }, treeState);
    } else {
      if (treeState != lastState) {
        lastState = treeState;
        // The change event will occur 1/2 the polling frequency before we
        // are aware of it, on average.
        lastChangeTime = Date.now() - 10000 / 2;
      }
      updateTimeBadge(lastChangeTime);
    }
  }

  function requestStatus() {
    sk.get('http://skia-tree-status.appspot.com/current?format=raw')
        .then(updateStatus)
        .catch(updateBadgeOnErrorStatus);
    setTimeout(requestStatus, 10000);
  }

  function main() {
    requestStatus();
  }

  main();

})();
