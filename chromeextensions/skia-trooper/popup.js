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

  function isLoggedIn() {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "https://alerts.skia.org/loginstatus/", false);
    xmlHttp.send(null);
    var loginStatus = JSON.parse(xmlHttp.responseText);
    return loginStatus["IsAGoogler"];
  }

  function getCommentInputField(alertId) {
    return "<input type='text' id='comment_input" + alertId + "' value=''>"
  }

  function getSnoozeSVG(alertId) {
    return "<svg id='snooze" + alertId + "' class='first-svg'><path d='M22 5.72l-4.6-3.86-1.29 1.53 4.6 3.86L22 5.72zM7.88 3.39L6.6 1.86 2 5.71l1.29 1.53 4.59-3.85zM12.5 8H11v6l4.75 2.85.75-1.23-4-2.37V8zM12 4c-4.97 0-9 4.03-9 9s4.02 9 9 9c4.97 0 9-4.03 9-9s-4.03-9-9-9zm0 16c-3.87 0-7-3.13-7-7s3.13-7 7-7 7 3.13 7 7-3.13 7-7 7z'><title>Snooze and add comment</title></path></svg>"
  }

  function getDismissSVG(alertId) {
    return "<svg id='dismiss" + alertId + "'><path d='M14.59 8L12 10.59 9.41 8 8 9.41 10.59 12 8 14.59 9.41 16 12 13.41 14.59 16 16 14.59 13.41 12 16 9.41 14.59 8zM12 2C6.47 2 2 6.47 2 12s4.47 10 10 10 10-4.47 10-10S17.53 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z'><title>Dismiss and add comment</title></path></svg>"
  }

  function getAddCommentSVG(alertId) {
    return "<svg id='addcomment" + alertId + "'><path d='M21.99 4c0-1.1-.89-2-1.99-2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h14l4 4-.01-18zM18 14H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z'><title>Add comment</title></path></svg>"
  }

  var bugRegex = new RegExp(".*Swarming bot (.*) is (quarantined|missing).*");

  var goloRegex = new RegExp(".*(a3|a4|m3|m5)");

  function getFileBugSVG(id) {
    var bugTemplate = `https://bugs.chromium.org/p/chromium/issues/entry?summary=[Device%20Restart]%20for%20${id}&description=Please%20Reboot%20${id}&cc=rmistry@google.com&components=Infra%3ELabs&labels=Pri-2,Infra-Troopers,Restrict-View-Google`
    return `<a href='${bugTemplate}' target='_blank' rel='noopener' class=auto-bug><svg id='file-bug'><path d='M20 8h-2.81c-.45-.78-1.07-1.45-1.82-1.96L17 4.41 15.59 3l-2.17 2.17C12.96 5.06 12.49 5 12 5c-.49 0-.96.06-1.41.17L8.41 3 7 4.41l1.62 1.63C7.88 6.55 7.26 7.22 6.81 8H4v2h2.09c-.05.33-.09.66-.09 1v1H4v2h2v1c0 .34.04.67.09 1H4v2h2.81c1.04 1.79 2.97 3 5.19 3s4.15-1.21 5.19-3H20v-2h-2.09c.05-.33.09-.66.09-1v-1h2v-2h-2v-1c0-.34-.04-.67-.09-1H20V8zm-6 8h-4v-2h4v2zm0-4h-4v-2h4v2z'><title>File a Chrome Infra bug</title></path></svg></a>`
  }

  function handleActionEvent(alertId, action) {
    var oneHourSnooze = Math.round((Date.now() + (60*60*1000))/1000);
    var commentText = document.getElementById("comment_input" + alertId).value;
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("POST", "https://alerts.skia.org/json/alerts/" + alertId + "/" + action, true);
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
    var body = JSON.stringify({until: oneHourSnooze, comment: commentText});
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
          document.getElementById(idName).innerHTML = data["username"].split("@")[0];
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

  function main() {
    var loginStatus = document.getElementById("login-status");
    if (! isLoggedIn()) {
      loginStatus.innerHTML = "(not logged in)"
    }
    setTrooperAndSheriff();

    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "https://alerts.skia.org/json/alerts/", true);
    xmlHttp.onload = function(e) {
      if (xmlHttp.readyState === 4) {
        if (xmlHttp.status === 200) {
          var alerts = JSON.parse(xmlHttp.responseText);

          var table = document.getElementById("alerts-table");

          var numSnoozedAlerts = 0;
          var foundActiveAlerts = false;
          // put alerts alphabetically for easier scanning.
          alerts.sort(function(a,b) {
            return a.name.localeCompare(b.name);
          });
          alerts.forEach(function(al) {
            if (al.category != "infra") {
              return;
            } else if (al.snoozedUntil != 0) {
              numSnoozedAlerts++;
              return;
            }
            foundActiveAlerts = true;

            var row = table.insertRow(-1);
            row.className = "alerts-row-name"
            var label = row.insertCell(-1);
            label.innerHTML = al.name;

            var row = table.insertRow(-1);
            row.className = "alerts-row-msg"
            var label = row.insertCell(-1);
            label.innerHTML = linkify(al.message);

            if (isLoggedIn()) {
              var alertId = al.id;
              var row = table.insertRow(-1);
              row.className = "alerts-row-actions"
              var label = row.insertCell(-1);
              label.innerHTML = getCommentInputField(alertId) + getSnoozeSVG(alertId) +
                                getDismissSVG(alertId) + getAddCommentSVG(alertId);

              var match = bugRegex.exec(al.name);
              if (match && goloRegex.exec(match[1])) {
                // match[1] is the bot id
                label.innerHTML += getFileBugSVG(match[1]);
              }
              // Add click listeners for all support actions.
              var actions = ["snooze", "dismiss", "addcomment"]
              actions.forEach(function(action) {
                document.getElementById(action + alertId).addEventListener(
                    "click", handleActionEvent.bind(this, alertId, action));
              });
            }
          });
          document.getElementById("snoozed-alerts").innerHTML =
              numSnoozedAlerts + " snoozed alerts";

          if (!foundActiveAlerts) {
            table.insertRow(-1).insertCell(-1).innerHTML = "<br/>No alerts are active."
          }
        } else {
          document.getElementById("errors").innerHTML += xmlHttp.statusText;
        }
      }
    }
    xmlHttp.onerror = function(e) {
      document.getElementById("errors").innerHTML += "Error getting alerts<br/>";
    }
    xmlHttp.send(null);
  }

  main();

})();
