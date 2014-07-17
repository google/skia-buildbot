/**
 * Skeleton to use as the basis for the refactoring of the current UI.
 *
 * The communication between parts of the code will be done by using Object.observe
 * on two global data structures. Well, at least global within this closure.
 *
 * The two data structures are 'traces' and 'queryInfo':
 *
 *   traces
 *     - A list of objects that can be passed directly to Flot for display.
 *   queryInfo
 *     - A list of all the keys and the parameters the user can search by.
 *
 * There are four objects that interact with those data structures:
 *
 * Plot
 *   - Handles plotting the data in traces via Flot.
 * Legend
 *   - Handles displaying the legend and turning traces on and off.
 * Query
 *   - Allows the user to select which traces to display.
 * Dataset
 *   - Allows the user to move among tiles, change scale, etc.
 *
 */
var skiaperf = (function() {
  // Plot and Legend watch traces.
  // Query and Legend can change traces.
  var traces = [
        /*
      {
        // All of these keys and values should be exactly what Flot consumes.
        data: [[1, 1.1], [20, 30]],
        label: "key1",
        color: "",
        show: false,
      },
      {
        data: [[1.2, 2.1], [20, 35]],
        label: "key2",
        color: "",
        show: false,
      },
        */
    ];

  // Query watches queryInfo.
  // Dataset can change queryInfo.
  var queryInfo = {
    allKeys: [
      /*
      ["desk_gmailthread.skp", "cpu",  "arm"],
      ["desk_gmailthread.skp", "cpu",  "x86"],
      ["desk_mapsvg.skp",      "wall", "x86"],
      */
    ],
    // The header name is the first value in each array.
    params: [
      /*
      ["benchName", "desk_gmailthread.skp", "desk_mapsvg.skp" ],
      ["timer", "wall", "cpu"], // 1
      ["arch", "arm7", "x86", "x86_64"], // 2
      */
    ]
  }

  function $$(query, par) {
    var id = function(e) { return e; };
    if(!par) {
      return Array.prototype.map.call(document.querySelectorAll(query), id);
    } else {
      return Array.prototype.map.call(par.querySelectorAll(query), id);
    }
  }

  function $$$(query, par) {
    return par ? par.querySelector(query) : document.querySelector(query);
  }

  function Plot() {
    new ArrayObserver(traces).open(function(slices) {
      $$$('#plot').textContent = "Selection has changed! " + traces.length;
    });
  }

  function Legend() {
    new ArrayObserver(traces).open(function(slices) {
    });
  }

  /**
   * Sets up the event handlers related to the query controls in the interface.
   * The callbacks in this function use and observe {@code queryInfo},
   * and modifies {@code traces}.
   */
  function Query() {
    /**
     * Syncs the DOM to match the current state of queryInfo.
     * It currently removes all the existing elements and then 
     * generates a new set that matches the queryInfo data.
     */
    function onParamChange() {
      console.log('onParamChange() triggered');
      var queryTable = $$$('#inputs');
      while(queryTable.hasChildNodes()) { 
        queryTable.removeChild(queryTable.lastChild);
      }

      for(var i = 0; i < queryInfo.params.length; i++) {
        var column = document.createElement('td');

        var longest = Math.max.apply(null, queryInfo.params[i].map(function(p) {
          return p.length;
        }));
        var minWidth = 0.75*longest + 0.5;

        var input = document.createElement('input');
        input.id = 'input_' + i;
        input.style.width = minWidth + 'em';

        var header = document.createElement('h4');
        header.innerText = queryInfo.params[i][0];

        var select = document.createElement('select');
        select.id = 'select_' + i;
        select.style.width = minWidth + 'em';
        select.style.overflow = 'auto';
        select.setAttribute('multiple', 'yes');

        for(var j = 1; j < queryInfo.params[i].length; j++) {
          var option = document.createElement('option');
          option.value = queryInfo.params[i][j];
          option.innerText = queryInfo.params[i][j].length > 0?
              queryInfo.params[i][j] : '(none)';
          select.appendChild(option);
        }

        column.appendChild(header);
        column.appendChild(input);
        column.appendChild(select);
        queryTable.appendChild(column);
      }
    }
    new ArrayObserver(queryInfo.params).open(onParamChange);
    new ArrayObserver(queryInfo.allKeys).open(onParamChange);
  }

  function Dataset() {
    var dataSet = "skps";
    var tileNum = [-1];
    var scale =  0;
  }

  /** microtasks
   *
   * Gets the Object.observe delivered.
   */
  function microtasks() {
    Platform.performMicrotaskCheckpoint();
    setTimeout(microtasks, 125);
  }

  function onLoad() {
    Plot();
    Legend();
    Query();
    Dataset();

    microtasks();
  };

  // If loaded via HTML Imports then DOMContentLoaded will be long done.
  if (document.readyState != "loading") {
    onLoad();
  } else {
    this.addEventListener('load', onLoad);
  }

  return {
    traces: traces,
    queryInfo: queryInfo
  };

}());
