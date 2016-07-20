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

  function main() {
    var xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", "https://alerts.skia.org/json/alerts/", true);
    xmlHttp.onload = function(e) {
      if (xmlHttp.readyState === 4) {
        if (xmlHttp.status === 200) {
          var alerts = JSON.parse(xmlHttp.responseText);

          var table = document.getElementById("alerts-table");

          var foundActiveAlerts = false;
          alerts.forEach(function(al) {
            if (al["category"] == "infra" && al["snoozedUntil"] == 0) {
              foundActiveAlerts = true;

              var row = table.insertRow(-1);
              row.className = "alerts-row-name"
              var label = row.insertCell(-1);
              label.innerHTML = al["name"]

              var row = table.insertRow(-1);
              row.className = "alerts-row-msg"
              var label = row.insertCell(-1);
              label.innerHTML = linkify(al["message"]);
            }
          });

          if (!foundActiveAlerts) {
            table.insertRow(-1).insertCell(-1).innerHTML = "No alerts are active."
          }
        } else {
          document.getElementById("errors").innerHTML = xmlHttp.statusText;
        }
      }
    }
    xmlHttp.onerror = function(e) {
      document.getElementById("errors").innerHTML = xmlHttp.statusText;
    }
    xmlHttp.send(null);
  }

  main();

})();
