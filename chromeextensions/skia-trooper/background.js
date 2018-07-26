// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

function getActiveInfraAlerts(data) {
  let numActiveInfraAlerts = 0;
  const alertGroups = JSON.parse(data).data;
  alertGroups.forEach(alertGroup => {
    if (!alertGroup.blocks) {
      return;
    }
    alertGroup.blocks.forEach(block => {
      block.alerts.forEach(al => {
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
  return numActiveInfraAlerts;
}

function updateIcon(numActiveInfraAlerts) {
  chrome.browserAction.setIcon({path:"bell.png"});
  chrome.browserAction.setTitle({title:"Alerts for Skia Troopers"});
  chrome.browserAction.setBadgeText({text: String(numActiveInfraAlerts)});
  if (numActiveInfraAlerts > 0) {
    chrome.browserAction.setBadgeBackgroundColor({color: "red"});
  } else {
    chrome.browserAction.setBadgeBackgroundColor({color: "green"});
  }
}

function logAlertServerErrors(e) {
  console.error("Error talking to alertserver");
  console.error(e);
}

function requestData() {
  sk.get("https://promalerts.skia.org/api/v1/alerts/groups")
    .then(data => {
      const numAlerts1 = getActiveInfraAlerts(data);
      sk.get("https://alerts2.skia.org/api/v1/alerts/groups")
        .then(data => {
          const numAlerts2 = getActiveInfraAlerts(data);
          updateIcon(numAlerts1 + numAlerts2);
        }).catch(e => logAlertServerErrors(e));
    }).catch(e => logAlertServerErrors(e));
  setTimeout(requestData, 10000);
}

requestData();
