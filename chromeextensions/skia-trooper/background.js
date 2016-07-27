// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function updateIcon() {
  var xmlHttp = new XMLHttpRequest();
  xmlHttp.open("GET", "https://alerts.skia.org/json/alerts/", true);
  xmlHttp.timeout = 5000;
  xmlHttp.onload = function(e) {
    if (xmlHttp.readyState === XMLHttpRequest.DONE && xmlHttp.status === 200) {
      var alerts = JSON.parse(xmlHttp.responseText);
      var numActiveInfraAlerts = 0;
      for (var i = 0; i < alerts.length; i++) {
        if (alerts[i]["category"] == "infra" && alerts[i]["snoozedUntil"] == 0) {
          numActiveInfraAlerts++;
        }
      }
      chrome.browserAction.setIcon({path:"bell.png"});
      chrome.browserAction.setTitle({title:"Alerts for Skia Troopers"});
      chrome.browserAction.setBadgeText({text: String(numActiveInfraAlerts)});
      if (numActiveInfraAlerts > 0) {
        chrome.browserAction.setBadgeBackgroundColor({color: "red"});
      } else {
        chrome.browserAction.setBadgeBackgroundColor({color: "green"});
      }
    } else {
      console.error("Error talking to alertserver.");
    }
  }
  xmlHttp.onreadystatechange = function() {
    if (xmlHttp.readyState == XMLHttpRequest.DONE) {
      setTimeout(updateIcon, 10000);
    }
  }
  xmlHttp.onerror = function(e) {
    console.error("Error talking to alertserver.");
  }
  xmlHttp.send(null);
}

updateIcon();
