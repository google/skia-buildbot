// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){


var bugRegex = new RegExp(".*Swarming bot (.*) is (quarantined|missing).*")
var goloRegex = new RegExp(".*(a3|a4|m3|m5)")

var bugPath = "M20 8h-2.81c-.45-.78-1.07-1.45-1.82-1.96L17 4.41 15.59 3l-2.17 2.17C12.96 5.06 12.49 5 12 5c-.49 0-.96.06-1.41.17L8.41 3 7 4.41l1.62 1.63C7.88 6.55 7.26 7.22 6.81 8H4v2h2.09c-.05.33-.09.66-.09 1v1H4v2h2v1c0 .34.04.67.09 1H4v2h2.81c1.04 1.79 2.97 3 5.19 3s4.15-1.21 5.19-3H20v-2h-2.09c.05-.33.09-.66.09-1v-1h2v-2h-2v-1c0-.34-.04-.67-.09-1H20V8zm-6 8h-4v-2h4v2zm0-4h-4v-2h4v2z";

function addBugsForGolo() {
  var alerts = document.getElementsByTagName("alert-sk");

  for(var i = 0; i< alerts.length; i++) {
    var alert = alerts[i];
    if (alert.getElementsByClassName("auto-bug").length) {
      // we already bugged it.
      continue;
    }
    var message = alert.getElementsByClassName("message")[0];
    var match = bugRegex.exec(message.textContent);
    if (!match) {
      continue;
    }
    if (goloRegex.exec(match[1])) {
      var title=message.getElementsByTagName("h3")[0];
      var id = match[1];
      var bugTemplate = `https://bugs.chromium.org/p/chromium/issues/entry?summary=[Device%20Restart]%20for%20${id}&description=Please%20Reboot%20${id}&cc=rmistry@google.com&components=Infra%3ELabs&labels=Pri-2,Infra-Troopers,Restrict-View-Google`;

      var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      svg.setAttributeNS(null, "width", "28");
      svg.setAttributeNS(null, "height", "28");
      svg.setAttributeNS(null, "preserveAspectRatio", "xMidYMid meet");
      svg.setAttributeNS(null, "viewBox", "0 0 24 24");

      var path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttributeNS(null, "d", bugPath);
      svg.appendChild(path);

      var anchor = document.createElement("a");
      anchor.classList.add("auto-bug");
      anchor.setAttribute("href", bugTemplate);
      anchor.setAttribute("target", "_blank");
      anchor.setAttribute("rel", "noopener");
      anchor.appendChild(svg);
      title.appendChild(anchor);
    }
  }
}

window.addEventListener("WebComponentsReady", function(e) {
  addBugsForGolo();
  // refresh every minute
  window.setInterval(addBugsForGolo, 60000);

});
})()