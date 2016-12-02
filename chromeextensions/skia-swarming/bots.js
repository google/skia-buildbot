// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

(function(){

// Doing a conditional write allows selecting, copying and paste to work
// instead of the selection constantly going away.
function overwriteIfDifferent(elem, str) {
  if (elem.innerHTML !== str) {
    elem.innerHTML = str;
  }
}

function drawExtraTableEntries(location, ssh, ifBroken) {
  var elem = document.getElementById("extra_location");
  if (location) {
    overwriteIfDifferent(elem, `<td>Physical Location</td><td colspan="2">${location}</td>`);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_ssh");
  if (ssh) {
    overwriteIfDifferent(elem, `<td>To SSH:</td><td colspan="2">${ssh}</td>`);
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_ifBroken");
  if (ifBroken) {
    overwriteIfDifferent(elem, `<td>If can't SSH:</td><td colspan="2">${ifBroken}</td>`);
  } else {
    elem.innerHTML = "";
  }
}

window.addEventListener("WebComponentsReady", function(e) {
  // Create our extra <tr> elements.
  var botTable = document.getElementsByTagName("table")[0];
  var newRow = document.createElement("tr");
  newRow.className = "outline";
  newRow.id="extra_location";
  botTable.children[0].appendChild(newRow);
  newRow = document.createElement("tr");
  newRow.className = "outline";
  newRow.id="extra_ssh";
  botTable.children[0].appendChild(newRow);
  newRow = document.createElement("tr");
  newRow.className = "outline";
  newRow.id="extra_ifBroken";
  botTable.children[0].appendChild(newRow);

  window.setInterval(function(){
    var id = document.getElementById("input").value;
    var state = document.getElementsByClassName("bot_state")[0];
    state = state.textContent.trim();
    if (!state || !id) {
      return;
    }
    state = JSON.parse(state) || {};

    var location = "";
    var ifBroken = "";
    var ssh = "";

    for (r in botMapping.locations) {
      if (id.match(r)) {
        location = botMapping.locations[r];
        break;
      }
    }
    for (r in botMapping.ifBroken) {
      if (id.match(r)) {
        ifBroken = botMapping.ifBroken[r];
        var idRegex = /_id_/g;
        ifBroken = ifBroken.replace(idRegex, id);
        break;
      }
    }
    for (r in botMapping.useJumphost) {
      if (id.match(r)) {
        ssh = `ssh -t -t chrome-bot@$JUMPHOST_IP "${state.ip}"`;
        break;
      }
    }
    for (r in botMapping.golo) {
      if (id.match(r)) {
        ssh = `ssh ${id}.golo <a href="https://chrome-internal.googlesource.com/infra/infra_internal/+/master/doc/ssh.md">For Access</a>`;
        break;
      }
    }
    for (r in botMapping.chrome) {
      if (id.match(r)) {
        ssh = `ssh ${id}.chrome <a href="https://chrome-internal.googlesource.com/infra/infra_internal/+/master/doc/ssh.md">For Access</a>`;
        break;
      }
    }
    for (r in botMapping.cloudConsole) {
      if (id.match(r)) {
        ssh = `SSH via Cloud Console (see above)`;
        break;
      }
    }
    if (ifBroken && !ssh) {
      ssh = "unreachable by ssh";
    }

    drawExtraTableEntries(location, ssh, ifBroken);
  }, 100);

});

})();