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


  /**
    * beginClustering by clearing out the old results and starting the XHR
    * request to get new clustering results.
    */
  function beginClustering(k, stddev, issue, selections) {
    sk.clearChildren($$$('#clResults'));
    // Results are always returned sorted vis StepRegression.
    $$$('input[value="stepregression"]').checked = true;

    document.body.style.cursor = 'wait';
    var url = '/clustering/?_k=' + k + '&_stddev=' + stddev + '&_issue=' + issue + '&' + selections;
    sk.get(url).then(JSON.parse).then(function(json) {
      var container = $$$('#clResults');
      json.Clusters.forEach(function(c){
        var sum = document.createElement('cluster-summary-sk');
        container.appendChild(sum);
        sum.summary = c;
      });
      document.body.style.cursor = 'auto';
    }).catch(function(e){
      alert(e);
      document.body.style.cursor = 'auto';
    });
  };

  // sort the clustering results with the algorithm given in element e.
  function sort(e) {
    if (!e.target.value) {
      return;
    }
    var sortBy = e.target.value;
    var container = $$$("#clResults");
    var to_sort = [];
    $$('cluster-summary-sk', container).forEach(function(ele) {
      to_sort.push({
        value: ele.dataset[sortBy],
        node: ele
      });
    });
    to_sort.sort(function(x, y) {
      return x.value - y.value;
    });
    to_sort.forEach(function(i) {
      container.appendChild(i.node);
    });
  };


  function onLoad() {
    var query = new sk.Query(queryInfo__);
    query.attach();

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
      beginClustering(
          $$$('#_k').value, $$$('#_stddev').value, $$$('#_issue').value, query.selectionsAsQuery());
    });

    $$('input[name="sort"]').forEach(function(ele) {
      ele.addEventListener('click', sort);
    });
  };

  // TODO(jcgregorio) Make this into a Promise.
  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

})();
