// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

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
    var perfURL = 'https://status.skia.org/json/perfAlerts';
    sk.get(perfURL).then(function(resp) {
      var numAlerts = JSON.parse(resp).alerts;
      document.getElementById('perf-alerts').innerHTML =
        linkify(numAlerts, 'https://perf.skia.org/alerts/');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to perfAlerts</br>';
    });
  }

  function setGoldAlerts() {
    var goldURL = 'https://status.skia.org/json/goldStatus';
    sk.get(goldURL).then(function(resp) {
      var json = JSON.parse(resp);
      document.getElementById('gm-alerts').innerHTML =
          linkify(json.gm, 'https://gold.skia.org/list?query=source_type%3Dgm');
      document.getElementById('image-alerts').innerHTML =
          linkify(json.image, 'https://gold.skia.org/list?query=source_type%3Dimage');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to goldStatus</br>';
    });
  }

  function setRollStatus() {
    var rollURL = 'https://autoroll.skia.org/json/status';
    sk.get(rollURL).then(function(resp) {
      var json = JSON.parse(resp);
      var currentStatus = json.currentRoll ? json.currentRoll.result : 'up to date';
      document.getElementById('current-roll').innerHTML =
          linkify(currentStatus, 'https://autoroll.skia.org/');
      document.getElementById('last-roll').innerHTML =
          linkify(json.lastRoll.result, 'https://autoroll.skia.org/');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to autoroller</br>';
    });
  }

  function setInfraAlerts() {
    var alertsURL = 'https://alerts.skia.org/json/alerts/';
    sk.get(alertsURL).then(function(resp) {
      var alerts = JSON.parse(resp);
      var activeInfraAlerts = 0;
      for (var i = 0; i < alerts.length; i++) {
        if (alerts[i]['category'] == 'infra' && alerts[i]['snoozedUntil'] == 0) {
          activeInfraAlerts++;
        }
      }
      document.getElementById('infra-alerts').innerHTML =
        linkify(activeInfraAlerts, 'http://alerts.skia.org/infra');
    }).catch(function() {
      document.getElementById('errors').innerHTML += 'Error connecting to alertserver</br>';
    });
  }

  function addMasterStatusRow(master) {
    var table = document.getElementById('status-table');

    var baseURL = 'http://build.chromium.org/p/' + master;
    var consoleURL = baseURL + '/console';
    var statusURL = baseURL + '/horizontal_one_box_per_builder';

    var row = table.insertRow(-1);
    row.className = 'trunk-status-row ' + master;
    var label = row.insertCell(-1);
    label.className = 'status-label';
    var labelAnchor = document.createElement('a');
    labelAnchor.href = consoleURL;
    labelAnchor.target = '_blank';
    labelAnchor.id = 'link_' + master;
    labelAnchor.textContent = master;
    label.appendChild(labelAnchor);

    var status = row.insertCell(-1);
    status.className = 'trunk-status-cell';
    var statusIframe = document.createElement('iframe');
    statusIframe.scrolling = 'no';
    statusIframe.src = statusURL;
    status.appendChild(statusIframe);
  }

  function addMasterStatusRows() {
    var masters = [
      'client.skia',
      'client.skia.android',
      'client.skia.compile',
      'client.skia.fyi'
    ];

    masters.forEach(function(master) {
      addMasterStatusRow(master);
    });
  }

  function main() {
    setTreeStatus();
    setTrooperSheriffRobocopWrangler();
    setPerfAlerts();
    setGoldAlerts();
    setRollStatus();
    setInfraAlerts();
    // addMasterStatusRows();
  }

  main();

})();
