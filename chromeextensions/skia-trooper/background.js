// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function updateIcon(data, appendToAlerts) {
  let numActiveInfraAlerts = 0;
  chrome.browserAction.getBadgeText({}, badgeText => {
    if (appendToAlerts) {
      numActiveInfraAlerts = Number(badgeText);
    }

    const alertGroups = JSON.parse(data).data;
    alertGroups.forEach(alertGroup => {
     if (!alertGroup.blocks) {
        return;
      }
      alertGroup.blocks.forEach(function(block) {
        block.alerts.forEach(function(al) {
          if (al.labels.category != "infra") {
            return;
          }
          if (al.status.state === "suppressed") {
            return;
          }
          numActiveInfraAlerts++;
        });
      });
    });
    chrome.browserAction.setIcon({path:"bell.png"});
    chrome.browserAction.setTitle({title:"Alerts for Skia Troopers"});
    chrome.browserAction.setBadgeText({text: String(numActiveInfraAlerts)});
    if (numActiveInfraAlerts > 0) {
      chrome.browserAction.setBadgeBackgroundColor({color: "red"});
    } else {
      chrome.browserAction.setBadgeBackgroundColor({color: "green"});
    }
  });
}

function requestData() {
  sk.get("https://promalerts.skia.org/api/v1/alerts/groups")
    .then(function(data) {
      updateIcon(data, false);
      sk.get("https://alerts2.skia.org/api/v1/alerts/groups")
        .then(data => updateIcon(data, true));
    }).catch(e => {
      console.error("Error talking to alertserver");
      console.error(e);
    });
  setTimeout(requestData, 10000);
}

requestData();
