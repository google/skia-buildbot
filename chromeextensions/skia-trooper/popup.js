// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

  function linkify(content) {
    var sub = '<a href="$&" target="_blank">$&</a>';
    content = content.replace(/https?:(\/\/|&#x2F;&#x2F;)[^ \t\n<]*/g, sub)
                     .replace(/(?:\r\n|\n|\r)/g, '<br/>');
    return content;
  }

  function loggedInAs() {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "https://skia.org/loginstatus/", false);
    xmlHttp.send(null);
    var loginStatus = JSON.parse(xmlHttp.responseText);
    return loginStatus["Email"];
  }

  function getCommentInputField(alertId) {
    return "<span><input type='text' id='comment_input" + alertId + "' value=''></span>"
  }

  function getSilenceRadio(alertId, displayName, value) {
    return "<input type='radio' name='duration" + alertId + "' id='duration" +
           alertId + "' value='" + value + "'>" + displayName;
  }

  function getSilenceButton(alertId) {
    return "<span class='left-padded'/><button type='button' id='button" + alertId + "'>Silence</button>";
  }

  var bugRegex = new RegExp(".*Swarming bot (.*) is (quarantined|missing).*");

  var goloRegex = new RegExp(".*(a9|m3|m5)");

  function getFileBugSVG(id) {
    var bugTemplate = `https://bugs.chromium.org/p/chromium/issues/entry?summary=[Machine%20Restart]%20for%20${id}&description=Please%20Reboot%20${id}&components=Infra%3ELabs&labels=Pri-2,Restrict-View-Google`
    return `<a href='${bugTemplate}' target='_blank' rel='noopener' class=auto-bug><svg id='file-bug'><path d='M20 8h-2.81c-.45-.78-1.07-1.45-1.82-1.96L17 4.41 15.59 3l-2.17 2.17C12.96 5.06 12.49 5 12 5c-.49 0-.96.06-1.41.17L8.41 3 7 4.41l1.62 1.63C7.88 6.55 7.26 7.22 6.81 8H4v2h2.09c-.05.33-.09.66-.09 1v1H4v2h2v1c0 .34.04.67.09 1H4v2h2.81c1.04 1.79 2.97 3 5.19 3s4.15-1.21 5.19-3H20v-2h-2.09c.05-.33.09-.66.09-1v-1h2v-2h-2v-1c0-.34-.04-.67-.09-1H20V8zm-6 8h-4v-2h4v2zm0-4h-4v-2h4v2z'><title>File a Chrome Infra bug</title></path></svg></a>`
  }

  function handleActionEvent(alertServer, alertId, labels) {
    var commentText = document.getElementById("comment_input" + alertId).value;
    var duration = parseInt(document.querySelector('input[name="duration' + alertId + '"]:checked').value);
    var endTime = new Date(Date.now() + duration);

    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("POST", "https://"+alertServer+"/api/v1/silences", true);
    xmlHttp.onload = function(e) {
      if (xmlHttp.readyState === 4 && xmlHttp.status === 200) {
        location.reload();
      } else {
        document.getElementById("errors").innerHTML += xmlHttp.statusText;
      }
    }
    xmlHttp.onerror = function(e) {
      document.getElementById("errors").innerHTML += "Error connecting to alerts server</br>";
    }
    var matchers = []
    for (var key in labels) {
      var matcher = {name: key, value: labels[key], type: 0}
      matchers.push(matcher);
    }
    var body = JSON.stringify({matchers: matchers, endsAt: endTime, comment: commentText, createdBy: loggedInAs()});
    xmlHttp.send(body);
  }

  function setTrooperAndSheriff() {
    var urls = ["http://skia-tree-status.appspot.com/current-trooper",
                "http://skia-tree-status.appspot.com/current-sheriff"]
    urls.forEach(function(url) {
      var xmlHttp = new XMLHttpRequest();
      xmlHttp.open("GET", url, true);
      xmlHttp.onload = function(e) {
        if (xmlHttp.readyState === 4 && xmlHttp.status === 200) {
          var data = JSON.parse(xmlHttp.responseText);
          var tokens = url.split("/");
          var idName = tokens[tokens.length - 1];
          var username = data["username"].split("@")[0];
          document.getElementById(idName).innerHTML =
              "<a href='http://who/" + username + "'>" + username + "</a>";
        } else {
          document.getElementById("errors").innerHTML += xmlHttp.statusText;
        }
      }
      xmlHttp.onerror = function(e) {
        document.getElementById("errors").innerHTML += "Error connecting to " + url + "</br>";
      }
      xmlHttp.send(null);
    });
  }

  function populateAlertsTable(alertServer, data, tableId, numSilencesId) {
    var resp = JSON.parse(data);
    var alertGroups = resp.data;

    var table = document.getElementById(tableId);

    var numSilencedAlerts = 0;
    var numActiveAlerts = 0;
    // Put alertGroups alphabetically for easier scanning.
    alertGroups.sort((a,b) => {
      return a.labels.alertname.localeCompare(b.labels.alertname);
    });

    alertGroups.forEach(alertGroup => {

      var groupName = alertGroup.labels.alertname;
      if (!alertGroup.blocks) {
        return;
      }
      alertGroup.blocks.forEach(block => {

        // Sort alerts in each group alphabetically for easier scanning
        // and consistent display.
        block.alerts.sort((a,b) => {
          return a.annotations.description.localeCompare(b.annotations.description);
        });

        block.alerts.forEach(al => {

          if (al.labels.category !== "infra") {
            return;
          }
          if (al.status.state === "suppressed") {
            numSilencedAlerts++;
            return
          }
          numActiveAlerts++;
          var alertId = numActiveAlerts;

          // Display the alert group name with button to silence.
          var row = table.insertRow(-1);
          row.className = "alerts-row-name"
          var label = row.insertCell(-1);
          label.innerHTML = groupName;
          var match = bugRegex.exec(al.annotations.description);
          if (match && goloRegex.exec(match[1])) {
            // match[1] is the bot id
            label.innerHTML += getFileBugSVG(match[1]);
          }

          // Display the alert message.
          var row = table.insertRow(-1);
          row.className = "alerts-row-msg"
          var label = row.insertCell(-1);
          label.innerHTML = linkify(al.annotations.description);

          // Add section for silences.
          var row = table.insertRow(-1);
          row.className = "alerts-row-silence"
          var label = row.insertCell(-1);
          label.innerHTML = getCommentInputField(alertId) +
                            getSilenceRadio(alertId, "1h", 1*60*60*1000) +
                            getSilenceRadio(alertId, "2h", 2*60*60*1000) +
                            getSilenceRadio(alertId, "24h", 24*60*60*1000) +
                            getSilenceButton(alertId);
          // Add click listener for the silence button.
          document.getElementById("button" + alertId).addEventListener(
              "click", handleActionEvent.bind(this, alertServer, alertId, al.labels));

        });
      });
    });
    document.getElementById(numSilencesId).innerHTML = numSilencedAlerts;

    if (!numActiveAlerts) {
      table.insertRow(-1).insertCell(-1).innerHTML = "<br/>No alerts are active."
    }
  }

  function displayAlertServerErrors(e, alertServer) {
    console.error(e);
    document.getElementById("errors").innerHTML += "Error getting alerts.<br/>Are you signed in with google.com account on "+alertServer+"?<br/>";
    document.getElementById("errors").innerHTML += e.response + "<br/>";
  }

  function populateAlerts(alertServer, tableId, numSilencesId) {
    sk.get("https://"+alertServer+"/api/v1/alerts/groups")
      .then(data => populateAlertsTable(alertServer, data, tableId, numSilencesId))
      .catch(e => displayAlertServerErrors(e, alertServer));
  }

  function main() {
    setTrooperAndSheriff();
    populateAlerts("promalerts.skia.org", "alerts-table1", "silenced-alerts1");
    populateAlerts("alerts2.skia.org", "alerts-table2", "silenced-alerts2");
  }

  main();

})();
