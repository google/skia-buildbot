// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

var stub = {
  locations: {
    "skia-rpi-0(0[1-9]|1[0-6])": "Skolo Shelf 3a",
    "skia-rpi-0(1[7-9]|2\d|3[0-2])": "Skolo Shelf 3b",

  },
  if_broken: {
    "skia-.*": "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>",
    "win.+": "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>",
  },
  use_jumphost: {
    "skia-rpi-.+": true,
    "skiabot-.+": true,
  },
  golo: {
    ".+(m3|m5)": true,
  },
  chrome: {
    ".+a3": true,
  }

}

var mapping = {};

function drawExtraTableEntries(location, ssh, if_broken) {
  var elem = document.getElementById("extra_location");
  if (location) {
    elem.innerHTML = `<td>Physical Location</td><td colspan=2>${location}</td>`;
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_ssh");
  if (ssh) {
    elem.innerHTML = `<td>To SSH:</td><td colspan=2>${ssh}</td>`;
  } else {
    elem.innerHTML = "";
  }

  elem = document.getElementById("extra_if_broken");
  if (if_broken) {
    elem.innerHTML = `<td>If can't SSH:</td><td colspan=2>${if_broken}</td>`;
  } else {
    elem.innerHTML = "";
  }
}

window.addEventListener("WebComponentsReady", function(e) {

  // Create our extra <tr> elements.
  var botTable = document.getElementsByTagName("table")[0];
  var test = document.createElement("tr");
  test.className = "outline";
  test.id="extra_location";
  botTable.children[0].appendChild(test);
  test = document.createElement("tr");
  test.className = "outline";
  test.id="extra_ssh";
  botTable.children[0].appendChild(test);
  test = document.createElement("tr");
  test.className = "outline";
  test.id="extra_if_broken";
  botTable.children[0].appendChild(test);

  // load in the mapping
  sk.get("https://skia.googlesource.com/buildbot/+/master/skolo/botmapping.json?format=TEXT").then(atob)
    .then(function(resp) {
    mapping = JSON.parse(resp);
  })
    .catch(function(err) {
      console.log(err);
      mapping = stub;
  });

  window.setInterval(function(){
    var id = document.getElementById("input").value;
    var state = document.getElementsByClassName("bot_state")[0];
    state = state.textContent.trim();
    if (!mapping.locations || !state || !id) {
      // hasn't loaded yet.
      return;
    }
    state = JSON.parse(state) || {};

    var location = "";
    var if_broken = "";
    var ssh = "";

    for (r in mapping.locations) {
      if (id.match(r)) {
        location = mapping.locations[r];
      }
    }
    for (r in mapping.if_broken) {
      if (id.match(r)) {
        if_broken = mapping.if_broken[r];
      }
    }
    for (r in mapping.use_jumphost) {
      if (id.match(r)) {
        ssh = `ssh -t -t chrome-bot@$JUMPHOST_IP "${state.ip}"`;
      }
    }
    for (r in mapping.golo) {
      if (id.match(r)) {
        ssh = `ssh ${id}.golo`;
      }
    }
    for (r in mapping.chrome) {
      if (id.match(r)) {
        ssh = `ssh ${id}.chrome`;
      }
    }
    if (if_broken && !ssh) {
      ssh = "unreachable by ssh";
    }

    drawExtraTableEntries(location, ssh, if_broken);
  }, 100);

});
