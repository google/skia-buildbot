// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

(function() {

  function onLoad() {
    document.body.style.cursor = 'wait';
    sk.get('/alerting/').then(JSON.parse).then(function(json) {
      var container = $$$('#alerts');
      sk.clearChildren(container);
      if (json.Clusters.length == 0) {
        container.innerHTML = "No active clusters exist.";
      } else {
        json.Clusters.forEach(function(c){
          var sum = document.createElement('cluster-summary-sk');
          container.appendChild(sum);
          sum.summary = c;
          sum.fade = true;
        });
      }
      document.body.style.cursor = 'auto';
    }).catch(function(e){
      document.body.style.cursor = 'auto';
    });
  };

  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

})();
