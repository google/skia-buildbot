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

  // doSort sorts the clustering results with the algorithm given in element e.
  function doSort(e) {
    if (!e.target.value) {
      return;
    }
    var container = $$$("#container");
    var to_sort = [];
    $$('div', container).forEach(function(ele) {
      var data = ele.dataset;
      if (!data.clustersize || !data.stepdeviation || !data.stepsize) {
        return;
      }
      to_sort.push([+data.clustersize, +data.stepdeviation, +data.stepsize, ele]);
    });
    switch(e.target.value) {
      case "stepFit":
        to_sort.sort(function(x, y) {
          return x[1]/x[2] - y[1]/y[2];
        });
        break;
      case "stepSize":
        to_sort.sort(function(x, y) {
          return y[2] - x[2];
        });
        break;
      case "stepDeviation":
        to_sort.sort(function(x, y) {
          return x[1] - y[1];
        });
        break;
      case "clusterSize":
      default:
        to_sort.sort(function(x, y) {
          return y[0] - x[0];
        });
    }
    to_sort.forEach(function(i) {
      container.appendChild(i[3]);
    });
  };

  function onLoad() {
    var query = new sk.Query(queryInfo__);
    query.attach();

    sk.get('/tiles/0/-1/').then(JSON.parse).then(function(json){
      queryInfo__.paramSet = json.paramset;
      queryInfo__.change.counter += 1;
    });

    $$$('#start').addEventListener('click', function(){
      // Build up query params and redirect.
      var url = '/clusters/?_k=' + $$$('#_k').value + '&_stddev=' + $$$('#_stddev').value + '&' + query.selectionsAsQuery();
      window.location.href = window.location.origin + url;
      // TODO(jcgregorio) This currently doesn't preserve the query selections.
    });

    $$('.expander').forEach(function(ele) {
      ele.addEventListener('click', function(e){
        e.target.nextElementSibling.classList.toggle("display");
      });
    });

    $$('input[name="sort"]').forEach(function(ele) {
      ele.addEventListener('click', doSort);
    });
  };

  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

})();
