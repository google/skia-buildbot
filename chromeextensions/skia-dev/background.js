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
    sk.get('http://tree-status.skia.org/current').then(function(statusResp) {
      var st = JSON.parse(statusResp);
      callback(new Date(st.date + ' UTC'));
    }).catch(updateBadgeOnErrorStatus);
  }

  var lastState;
  var lastChangeTime;
  function updateStatus(statusResp) {
    var st = JSON.parse(statusResp);
    var badgeState = {
      open: {color: 'green', defaultText: '\u2022'},
      closed: {color: 'red', defaultText: '\u00D7'},
      caution: {color: '#CDCD00', defaultText: '!'}
    };

    const msg = st.message;
    chrome.browserAction.setTitle({title:msg});
    var treeState = (/open/i).exec(msg) ? 'open' :
        (/caution/i).exec(msg) ? 'caution' : 'closed';

    if (lastState && lastState != treeState) {
      notifyStatusChange(treeState, msg);
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
    sk.get('http://tree-status.skia.org/current')
        .then(updateStatus)
        .catch(updateBadgeOnErrorStatus);
    setTimeout(requestStatus, 10000);
  }

  function main() {
    requestStatus();
  }

  main();

})();
