// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

  var MAX_SUBJECT_LENGTH = 40;

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
    var statusURL = 'http://skia-tree-status.appspot.com/allstatus?limit=1&format=json';
    sk.get(statusURL).then(function(resp) {
      var entry = JSON.parse(resp)[0];
      var d = new Date(entry.date + ' UTC');
      var delta = sk.human.diffDate(d);
      var messageLink = linkify(
          entry.message, 'http://skia-tree-status.appspot.com/');
      var statusSummary = '<b>' + messageLink + '</b> [' +
                          entry.username.split('@')[0] + ' ' + delta + ' ago]';
      document.getElementById('tree-status').innerHTML = 'Tree status: ' + statusSummary;
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to skia-tree-status</br>';
    });
  }

  function setTrooperSheriffRobocopWrangler() {
    var urls = ['http://skia-tree-status.appspot.com/current-trooper',
                'http://skia-tree-status.appspot.com/current-sheriff',
                'http://skia-tree-status.appspot.com/current-robocop',
                'http://skia-tree-status.appspot.com/current-gpu-sheriff']
    urls.forEach(function(url) {
      sk.get(url).then(function(resp) {
        var tokens = url.split('/');
        var idName = tokens[tokens.length - 1];
        var username = JSON.parse(resp).username.split('@')[0];
        var usernameLink = linkify(username, 'http://who/' + username);
        document.getElementById(idName).innerHTML = usernameLink;
      }).catch(function() {
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
    var goldURL = 'https://gold.skia.org/json/trstatus';
    sk.get(goldURL).then(function(resp) {
      var json = JSON.parse(resp);
      json.corpStatus.forEach(function(stat) {
        if (stat.name == "gm") {
          document.getElementById('gm-alerts').innerHTML =
              linkify(stat.untriagedCount, 'https://gold.skia.org/list?query=source_type%3Dgm');
        } else if (stat.name == "image") {
          document.getElementById('image-alerts').innerHTML =
              linkify(stat.untriagedCount, 'https://gold.skia.org/list?query=source_type%3Dimage');
        }
      });
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to goldStatus</br>';
    });
  }

  function setCrRollStatus() {
    var rollURL = 'https://autoroll.skia.org/json/status';
    sk.get(rollURL).then(function(resp) {
      var json = JSON.parse(resp);
      var currentStatus = json.currentRoll ? json.currentRoll.result : 'up to date';
      document.getElementById('current-cr-roll').innerHTML =
          linkify(currentStatus, 'https://autoroll.skia.org/');
      document.getElementById('last-cr-roll').innerHTML =
          linkify(json.lastRoll.result, 'https://autoroll.skia.org/');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to Cr autoroller</br>';
    });
  }

  function setAndroidRollStatus() {
    var rollURL = 'https://storage.googleapis.com/skia-android-autoroller/roll-status.json';
    sk.get(rollURL).then(function(resp) {
      var rolls = JSON.parse(resp).rolls;
      var currentStatus = rolls[0].status === 'succeeded' ? 'up to date' : rolls[0].status;
      var linkToRolls = 'https://googleplex-android-review.git.corp.google.com/q/' +
                        'owner:31977622648%40project.gserviceaccount.com';
      document.getElementById('current-android-roll').innerHTML =
          linkify(currentStatus, linkToRolls);
      document.getElementById('last-android-roll').innerHTML =
          linkify(rolls[1].status, linkToRolls);
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to Android autoroller</br>';
    });
  }

  function setInfraAlerts() {
    var alertsURL = 'https://promalerts.skia.org/api/v1/alerts/groups';
    sk.get(alertsURL).then(function(resp) {
      var activeInfraAlerts = 0;
      var json = JSON.parse(resp);
      var alertGroups = json.data;
      alertGroups.forEach(function(alertGroup) {
        if (!alertGroup.blocks) {
          return;
        }
        alertGroup.blocks.forEach(function(block) {
          block.alerts.forEach(function(al) {
            if (al.labels.category != "infra") {
              return;
            }
            if (al.silenced) {
              return;
            }
            activeInfraAlerts++;
          });
        });
      });
      document.getElementById('infra-alerts').innerHTML =
        linkify(activeInfraAlerts, 'http://alerts.skia.org/infra');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to alertserver</br>';
    });
  }

  function addGerritChanges() {
    var username = loggedInAs();
    if (!username) {
      document.getElementById('errors').innerHTML += 'Log in to skia.org to view your Gerrit changes</br>';
      return
    }

    var listChangesURL =
        "http://skia-review.googlesource.com/changes/?q=status:open+owner:" +
        loggedInAs() + "&o=LABELS";
    var inCQChanges = []
    var approvedChanges = []
    var waitingForApprovalChanges = []
    var wipChanges = []

    sk.get(listChangesURL).then(function(resp) {
      // Remove JSON anti-hijacking prefix.
      var responseText = resp.substring(')]}\'\n'.length);
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

      addChangesToTable('In Commit Queue', 'in-cq', inCQChanges)
      addChangesToTable('Approved', 'approved', approvedChanges)
      addChangesToTable('Under Review', 'under-review', waitingForApprovalChanges)
      addChangesToTable('WIP', 'wip', wipChanges)

    }).catch(function(err) {
      console.log(err);
      document.getElementById('errors').innerHTML += 'Error connecting to skia-review</br>';
    });

  }

  function addChangesToTable(state, stateClassName, changes, table) {
    if (!changes.length) {
      return
    }
    var table = document.getElementById('gerrit-changes');
    changes.forEach(function(change) {
      var changeRow = table.insertRow(-1);

      // Add link to the change
      var linkCol = changeRow.insertCell(-1);
      var linkAnchor = document.createElement('a');
      linkAnchor.href = 'https://skia-review.googlesource.com/c/' + change._number;
      linkAnchor.target = '_blank';
      linkAnchor.textContent = 'skrev/' + change._number;
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

      // Add state.
      var stateRow = changeRow.insertCell(-1);
      var stateSpan = document.createElement('span');
      stateSpan.className = stateClassName;
      stateSpan.textContent = state;
      stateRow.appendChild(stateSpan);
    });
  }

  function main() {
    setTreeStatus();
    setTrooperSheriffRobocopWrangler();
    setPerfAlerts();
    setGoldAlerts();
    setCrRollStatus();
    setAndroidRollStatus();
    setInfraAlerts();
    addGerritChanges();
  }

  main();

})();
