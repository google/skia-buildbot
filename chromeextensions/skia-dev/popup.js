// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

  var MAX_SUBJECT_LENGTH = 80;

  function loggedInAs() {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "https://skia.org/loginstatus/", false);
    xmlHttp.send(null);
    var loginStatus = JSON.parse(xmlHttp.responseText);
    return loginStatus["Email"];
  }

  function linkify(wrapMe, link) {
    return '<a href="' + link + '" target="_blank">' + wrapMe + '</a>'
  }

  function setTreeStatus() {
    var statusURL = 'http://tree-status.skia.org/current';
    sk.get(statusURL).then(function(resp) {
      var entry = JSON.parse(resp);
      var d = new Date(entry.date + ' UTC');
      var delta = sk.human.diffDate(d);
      var messageLink = linkify(
          entry.message, 'http://tree-status.skia.org/');
      var statusSummary = '<b>' + messageLink + '</b> [' +
                          entry.username.split('@')[0] + ' ' + delta + ' ago]';
      document.getElementById('tree-status').innerHTML = 'Tree status: ' + statusSummary;
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to tree-status</br>';
    });
  }

  function setRotations() {
    var urls = ['https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener',
                'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener',
                'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-android-gardener',
                'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gpu-gardener'];
    urls.forEach(function(url) {
      sk.get(url).then(function(resp) {
        var tokens = url.split(':');
        var idName = tokens[tokens.length - 1];
        // Skia rotations have a single primary.
        var username = JSON.parse(resp).emails[0].split('@')[0];
        var usernameLink = linkify(username, 'http://who/' + username);
        document.getElementById(idName).innerHTML = usernameLink;
      }).catch(function(err) {
        document.getElementById('errors').innerHTML += 'Error connecting to ' + url + '</br>';
      });
    });
  }

  function setPerfAlerts() {
    var perfURL = 'https://perf.skia.org/_/alerts/';
    sk.get(perfURL).then(function(resp) {
      var numAlerts = JSON.parse(resp).alerts;
      document.getElementById('perf-alerts').innerHTML =
        linkify(numAlerts, 'https://perf.skia.org/alerts/');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to perfAlerts</br>';
    });
  }

  function setGoldAlerts() {
    var goldURL = 'https://gold.skia.org/json/v1/trstatus';
    sk.get(goldURL).then(function(resp) {
      var json = JSON.parse(resp);
      json.corpStatus.forEach(function(stat) {
        if (stat.name === "gm") {
          document.getElementById('gm-alerts').innerHTML =
              linkify(stat.untriagedCount, 'https://gold.skia.org/list?query=source_type%3Dgm');
        } else if (stat.name === "image") {
          document.getElementById('image-alerts').innerHTML =
              linkify(stat.untriagedCount, 'https://gold.skia.org/list?query=source_type%3Dimage');
        }
      });
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to goldStatus</br>';
    });
  }

  function addGerritChanges() {
    var username = loggedInAs();
    if (!username) {
      document.getElementById('errors').innerHTML += 'Log in to skia.org to view your Gerrit changes</br>';
      return;
    }

    // Find the Gerrit account ID of the user to use for filtering later.
    var queryAccountURL =
        `http://skia-review.googlesource.com/accounts/?q=email:${username}`;
    sk.get(queryAccountURL).then(function(resp) {
      // Remove JSON anti-hijacking prefix.
      var responseText = resp.substring(")]}'\n".length);
      var json = JSON.parse(responseText);

      var userAccountId = "";
      json.forEach(function(accountSection) {
        userAccountId = accountSection._account_id;
      });
      if (!userAccountId) {
        document.getElementById('errors').innerHTML += 'Could not find accountId from skia-review</br>';
        return;
      }

      // Find all Gerrit changes that are waiting for this user to review.
      var reviewerChangesURL =
          `http://skia-review.googlesource.com/changes/?q=status:open+reviewer:${username}+
           -owner:${username}&o=DETAILED_LABELS`;
      var pendingReviewChanges = [];
      sk.get(reviewerChangesURL).then(function(resp) {
        // Remove JSON anti-hijacking prefix.
        var responseText = resp.substring(")]}'\n".length);
        var json = JSON.parse(responseText);

        json.forEach(function(change) {
          // See if you already have a +1.
          var approved = false;
          if (change.labels["Code-Review"] && change.labels["Code-Review"].all) {
            change.labels["Code-Review"].all.forEach(function(vote) {
              if (vote._account_id === userAccountId && vote.value === 1) {
                approved = true;
              }
            });
          }
          if (!approved) {
            pendingReviewChanges.push(change);
          }
        });
        addChangesToTable(pendingReviewChanges, "gerrit-reviewer", "Pending your review", "red",
                          `status:open+reviewer:${username}+-owner:${username}`);
      });
    }).catch(function(err) {
      console.log(err);
      document.getElementById('errors').innerHTML += 'Error connecting to skia-review</br>';
    });

    // Find all Gerrit changes owned by this user in different states.
    var listChangesURL =
        `http://skia-review.googlesource.com/changes/?q=status:open+owner:${username}&o=LABELS`;
    var inCQChanges = [];
    var approvedChanges = [];
    var waitingForApprovalChanges = [];
    var wipChanges = [];

    sk.get(listChangesURL).then(function(resp) {
      // Remove JSON anti-hijacking prefix.
      var responseText = resp.substring(")]}'\n".length);
      var json = JSON.parse(responseText);

      json.forEach(function(change) {
        if (change.has_review_started && !change.subject.startsWith("WIP: ")) {
          // Review has started so it is in one of 3 states:
          // * CQ running.
          // * Approved but not sent to CQ yet.
          // * Waiting for approval.
          if (change.labels) {
            if(change.labels["Commit-Queue"] && change.labels["Commit-Queue"].approved) {
              inCQChanges.push(change);
            } else if(change.labels["Code-Review"] && change.labels["Code-Review"].approved) {
              approvedChanges.push(change);
            } else {
              waitingForApprovalChanges.push(change);
            }
          }
        } else {
          // Review has not started so it is WIP.
          wipChanges.push(change);
        }
      });

      // Display all owned changes in the UI.
      addChangesToTable(inCQChanges, "gerrit-cq", "In CQ", "green",
                        `status:open+owner:${username}+label:Commit-Queue=2`);
      addChangesToTable(approvedChanges, "gerrit-approved", "Approved, ready to submit", "green",
                        `status:open+owner:${username}+-label:Commit-Queue=2+label:Code-Review=1`);
      addChangesToTable(waitingForApprovalChanges, "gerrit-under-review", "Under review", "yellow",
                        `status:open+-is:wip+owner:${username}+-label:Code-Review=1`);
      addChangesToTable(wipChanges, "gerrit-wip", "WIP", "yellow",
                        `status:open+is:wip+owner:${username}`);

    }).catch(function(err) {
      console.log(err);
      document.getElementById('errors').innerHTML += 'Error connecting to skia-review</br>';
    });

  }

  function addChangesToTable(changes, tableId, title, titleClassName, query) {
    if (!changes.length) {
      return;
    }
    var table = document.getElementById(tableId);

    // Add title row.
    var titleRow = table.insertRow(-1);
    var titleCol = titleRow.insertCell(-1);
    titleCol.colSpan = 2;
    titleCol.className = titleClassName;
    var titleBold = document.createElement('b');
    titleBold.textContent = title;
    var newLine = document.createElement('br');
    var titleQueryLink = document.createElement('span');
    titleQueryLink.innerHTML = `(<a target="_blank" href="https://skia-review.googlesource.com/q/${query}">query</a> limited to 5)`;
    titleCol.appendChild(titleBold);
    titleCol.appendChild(newLine);
    titleCol.appendChild(titleQueryLink);

    // Truncate number of changes to 5.
    changes = changes.splice(0, 5);
    changes.forEach(function(change) {
      var changeRow = table.insertRow(-1);

      // Add link to the change
      var linkCol = changeRow.insertCell(-1);
      var linkAnchor = document.createElement('a');
      linkAnchor.href = `https://skia-review.googlesource.com/c/${change._number}`;
      linkAnchor.target = '_blank';
      linkAnchor.textContent = `skrev/${change._number}`;
      linkCol.appendChild(linkAnchor);

      // Add subject.
      var subjRow = changeRow.insertCell(-1);
      var subjSpan = document.createElement('span');
      if (change.subject.length > MAX_SUBJECT_LENGTH) {
        subjSpan.textContent =
            change.subject.substring(0, MAX_SUBJECT_LENGTH) + "...";
      } else {
        subjSpan.textContent = change.subject;
      }
      subjRow.appendChild(subjSpan);
    });

    table.parentNode.insertBefore(document.createElement('br'), table);
    table.parentNode.insertBefore(document.createElement('br'), table);
  }

  function main() {
    setTreeStatus();
    setRotations();
    setPerfAlerts();
    setGoldAlerts();
    addGerritChanges();
  }

  main();

})();
