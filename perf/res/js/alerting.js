// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

(function() {

  var clusterSummaries = [];

  var intervalID = null;

  function onLoad() {
    refreshAlerts();
    intervalID = setInterval(refreshAlerts, 60*1000);
    $$$('#autoRefresh').addEventListener('click', function(e) {
      if (e.checked) {
        intervalID = setInterval(refreshAlerts, 60*1000);
      } else {
        clearInterval(intervalID);
      }
    });
  }


  function refreshAlerts() {
    document.body.style.cursor = 'wait';
    sk.get('/alerting/').then(JSON.parse).then(function(json) {
      clusterSummaries.forEach(function(c) {
        c.detach();
      });
      var container = $$$('#alerts');
      sk.clearChildren(container);
      if (json.Clusters.length == 0) {
        container.innerHTML = "No active clusters exist.";
      } else {
        json.Clusters.forEach(function(c){
          var summary = new sk.ClusterSummary(container, c);
          summary.attach();
          clusterSummaries.push(summary);
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
