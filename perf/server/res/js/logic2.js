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
(function() {
  // Plot and Legend watch traces.
  // Query and Legend can change traces.
  var traces = [
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
    ];

  // Query watches queryInfo.
  // Dataset can change queryInfo.
  var queryInfo = {
    allKeys: [
      ["desk_gmailthread.skp", "cpu",  "arm"],
      ["desk_gmailthread.skp", "cpu",  "x86"],
      ["desk_mapsvg.skp",      "wall", "x86"],
    ],
    // The header name is the first value in each array.
    params: [
      ["benchName", "desk_gmailthread.skp", "desk_mapsvg.skp" ],
      ["timer", "wall", "cpu"], // 1
      ["arch", "arm7", "x86", "x86_64"], // 2
    ]
  }

  function $$(query, par) {
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
      $$$('#legend').textContent = "Selection has changed! " + traces.length;
    });

    $$$('#removeTrace').addEventListener('click', function() {
      traces.splice(0, 1);
    });
    $$$('#toggleTrace').addEventListener('click', function() {
      var t = traces[0];
      traces[0] = {
        data: t.data,
        label: t.label,
        color: t.color,
        show: !t.show
      };
    });
  }

  function Query() {
    function onParamChange() {
      $$$('#query').textContent = "Params have changed!";
    }
    new ArrayObserver(queryInfo.params).open(onParamChange);
    new ArrayObserver(queryInfo.allKeys).open(onParamChange);

    var i = 0;
    $$$('#addTrace').addEventListener('click', function() {
      traces.push(
        {
          data: [[1.2, 2.1], [20, 35]],
          label: "key" + i,
          color: "",
          show: false,
        });
      i += 1;
    });
  }

  function Dataset() {
    var dataSet = "skps";
    var tileNum = [0, 1];
    var scale =  0;

    $$$('#changeTile').addEventListener('click', function() {

      traces.splice(0, traces.length,
        {
          data: [[1.2, 2.1], [20, 35]],
          label: "key",
          color: "",
          show: false,
        });
      queryInfo.params.push(["a", "b", "c"]);
    });
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

}());
