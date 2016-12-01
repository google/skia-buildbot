// Copyright (c) 2016 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

var stub = {
  "skia-rpi-001": {
    location: "Skolo Shelf 3a",
    jumphost_ip: "192.168.1.101",
    if_broken: "<a href='https://goto.google.com/skolo-maintenance'>go/skolo-maintenance</a>"
  },
}

window.addEventListener('WebComponentsReady', function(e) {

  console.log("hello bots");
  var mapping = {};

  sk.get("https://skia.googlesource.com/buildbot/+/master/skolo/botmapping.json?format=TEXT").then(atob)
    .then(function(resp) {
    mapping = JSON.parse(resp);
  })
    .catch(function(err) {
      console.log(err);
      mapping = stub;
  }).then(function(){
    var id = document.getElementById("input").value;

    if (mapping[id]) {
      var info = mapping[id];
      var botTable = document.getElementsByTagName("table")[0];
      var test = document.createElement("tr");
      test.className = "outline";
      test.innerHTML = `<td>Physical Location</td><td colspan=2>${info.location}</td>`;
      botTable.children[0].appendChild(test);

      test = document.createElement("tr");
      test.className = "outline";
      if (info.jumphost_ip) {
        test.innerHTML = `<td>To SSH:</td><td colspan=2>ssh -t -t chrome-bot@$JUMPHOST_IP "${info.jumphost_ip}"</td>`;
      }

      botTable.children[0].appendChild(test);

      test = document.createElement("tr");
      test.className = "outline";
      test.innerHTML = `<td>If can't SSH:</td><td colspan=2>${info.if_broken}</td>`;
      botTable.children[0].appendChild(test);
    }
  });

});
