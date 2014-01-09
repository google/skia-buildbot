// Copyright (c) 2012 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

/**
 * @fileoverview Code related to the popup page. Primarily calls out to the
 * background page for data on the status of the buildbots.
 */



var bg = chrome.extension.getBackgroundPage();
var statusLinkBase = bg.BASE_URL;
console.log(statusLinkBase);
var failuresParent = document.getElementById('bots');

if (bg.botStatusList) {

  for (var i = 0; i < bg.botStatusList.length; i++) {
    var botStatus = bg.botStatusList[i];
    if (botStatus.success)
      continue;
    var botDiv = document.createElement('div');
    botDiv.className = 'bot failure';
    botDiv.innerHTML = '<a href="' + statusLinkBase
      + botStatus.statusLink + '" target="_blank">' + botStatus.name + '</a>';
    failuresParent.appendChild(botDiv);
  }
}

var linksDiv = document.getElementById('links');
linksDiv.innerHTML = 
    '<a href="https://rawgithub.com/google/skia-buildbot/master/buildbots.html" ' +
    'target=_blank>Console</a>&nbsp;<a href="' + statusLinkBase + 
    'waterfall" target=_blank>Waterfall</a>';
