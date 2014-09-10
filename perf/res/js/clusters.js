// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

(function() {


  var queryInfo__ = {
    /**
     * Contains an array of arrays, each array representing a single parameter
     * that can be set, each element a different possibility of what to set it
     * to.
     */
    paramSet: [
      /*
       "benchName": ["desk_gmailthread.skp", "desk_mapsvg.skp" ],
       "timer":     ["wall", "cpu"],
       "arch":      ["arm7", "x86", "x86_64"],
       */
      ],
    // change is used because Observe-js has trouble dealing with the large
    // array changes that happen when Navigation swaps paramSet data.
    change: {
      counter: 0
    },
  };


  function onLoad() {
    var query = new sk.Query(queryInfo__);
    query.attach();

    var cluster = new sk.Cluster();
    cluster.attach();

    sk.get('/tiles/0/-1/').then(JSON.parse).then(function(json){
      queryInfo__.paramSet = json.paramset;
      queryInfo__.change.counter += 1;
    });

    sk.get('/trybots/').then(JSON.parse).then(function(json){
      var select = $$$('#_issue');
      json.forEach(function(issue) {
        var op = document.createElement('OPTION');
        op.value = issue;
        op.innerText = issue;
        select.appendChild(op);
      });
    });

    $$$('#start').addEventListener('click', function(){
      cluster.beginClustering($$$('#_k').value, $$$('#_stddev').value, $$$('#_issue').value, query.selectionsAsQuery());
    });
  };

  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

})();
