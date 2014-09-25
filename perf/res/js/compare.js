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
    * beginCompare starts an XHR request to populate variables used by
    * sk.Compare.display(), then calls it to display compare results.
    *
    * compare: the sk.Compare object.
    * vertical: the trace param criterion we are comparing in, e.g., "config".
    * selections: the query selection result as url fragment.
    *
    * commit: string of commit hash. If invalid or not found, will use latest
    * available data.
    *
    * See query and compare modules for more details.
    */
  function beginCompare(compare, vertical, selections, commit) {
    // The pair of vertical values to compare against.
    var pair = [
      /*
       "8888", "gpu",
       */
      ];
    // List of table columns.
    var cols = [
      /*
       "x86:ANGLE:HD2000:ShuttleA:Win7",
       "x86:GTX660:ShuttleA:Ubuntu12",
       */
      ];
    // cells contains data for the table cells, keyed by test name as row name.
    // Within each row, keys are the corresponding column indices in the cols
    // array above and values are the bench values corresponding to the pair of
    // verticals above.
    var cells = {
      /*
        "Deque_Push_640_480": {
          "0": [0.9, 1.3],
          "2": [1.1, 0.8],
          ...
        },
        "Another_Test": {
          "1": [0, 0.7],
          ...
        }
       */
      };

    document.body.style.cursor = 'wait';
    var re = new RegExp(vertical + '=', 'g');
    var inparam = selections.match(re);
    if (!inparam || inparam.length != 2) {
      alert("Please select EXACTLY two elements from " + vertical);
      document.body.style.cursor = 'auto';
      return;
    }
    var col_params = ['arch', 'config', 'extra_config', 'gpu', 'model', 'os'];
    sk.get('/single/' + commit + '?' + selections).then(JSON.parse).then(function(json) {
      json.traces.forEach(function(tr) {
        var params = tr['params'];
        if (pair.indexOf(params[vertical]) < 0) {
          pair.push(params[vertical]);
        }
        // Constructs the column string.
        var col = col_params.map(function(key) {
          if (key != vertical) {
            return params[key];
          } else {
            return '';
          }
        }).filter(function(v) {
          return (v && v.length > 0);
        }).join(':');
        if (cols.indexOf(col) < 0) {
          cols.push(col);
        }
        // Records new data.
        var row = params['test'];
        if (!(row in cells)) {
          cells[row] = {};
        }
        var colIdxStr = cols.indexOf(col).toString();
        if (!(colIdxStr in cells[row])) {
          cells[row][colIdxStr] = [0, 0];
        }
        var pairIdx = pair.indexOf(params[vertical]);
        if (cells[row][colIdxStr][pairIdx] !== 0) {
          // row/col combination not unique: something wrong with the data.
          alert('Duplicate data for ' + row + ':' + col);
          document.body.style.cursor = 'auto';
          return;
        }
        // Records the latest value.
        cells[row][colIdxStr][pairIdx] = tr['val'];
      });

      if (cols.length == 0 || pair.length != 2) {
        $$$('#compStatus').innerText = 'No data row or column.';
        sk.clearChildren(compare.resultsContainer_);
      } else {
        $$$('#compStatus').innerText = 'Comparing "' + vertical + '", ratio ' +
            pair[1] + ' / ' + pair[0] + ':';
        compare.display(cols, cells);
      }
      document.body.style.cursor = 'auto';
    }).catch(function(e){
      alert(e);
      document.body.style.cursor = 'auto';
    });
  }

  /**
   * populateAndSetCommits populates latest commit info into the select DOM.
   *
   * selectDom: a given <select> DOM.
   *
   * options are sorted by latest commits first. Option's value is the commit's
   * hash in the tile commit data.
   *
   * TODO: move to a common file?
   */
  function populateAndSetCommits(selectDom, beginHash, endHash) {
    if (selectDom.nodeName != 'SELECT') {
      alert('Cannot populate non-SELECT DOM!');
      return;
    }
    sk.clearChildren(selectDom);
    sk.get('/shortcommits/?begin=' + beginHash + '&end=' + endHash).then(JSON.parse).then(function(json){
      for (var i = 0; i < json.Commits.length; i++) {
        var c = json.Commits[i];
        var opt = document.createElement('OPTION');
        opt.value = c.Hash;
        opt.innerText = [c.Hash.substring(0, 7), c.Author,
            c.Subject.substr(0, 80)].join(':');
        selectDom.insertBefore(opt, selectDom.firstChild);
      }
    });
  }

  function onLoad() {
    var query = new sk.Query(queryInfo__);
    query.attach();

    var compare = new sk.Compare();
    compare.attach();

    sk.get('/tiles/0/-1/').then(JSON.parse).then(function(json){
      queryInfo__.paramSet = json.paramset;
      queryInfo__.change.counter += 1;
      var endCommit = "";
      for (i = json.commits.length - 1; i >= 0; i--) {
        if (json.commits[i].hash.length > 0) {
          endCommit = json.commits[i].hash;
          break;
        }
      }
      populateAndSetCommits($$$('#commitSel'), json.commits[0].hash, endCommit);
    });

    $$$('#start').addEventListener('click', function(){
      beginCompare(compare, $$$('input[name="vertical"]:checked').value, query.selectionsAsQuery(), $$$('#commitSel').value);
    });

    $$$('#commitSel').addEventListener('change', function(){
      beginCompare(compare, $$$('input[name="vertical"]:checked').value, query.selectionsAsQuery(), $$$('#commitSel').value);
    });
  };

  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

})();
