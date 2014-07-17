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
        lines: {
          show: false
        }
      },
      {
        data: [[1.2, 2.1], [20, 35]],
        label: "key2",
        color: "",
        lines: {
          show: false
        }
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
  };

  function $$(query, par) {
    var id = function(e) { return e; };
    if (!par) {
      return Array.prototype.map.call(document.querySelectorAll(query), id);
    } else {
      return Array.prototype.map.call(par.querySelectorAll(query), id);
    }
  }

  function $$$(query, par) {
    return par ? par.querySelector(query) : document.querySelector(query);
  }

  /**
   * Sets up the callbacks related to the plot.
   * Plot observes traces.
   */
  function Plot() {

    /**
     * Reference to the underlying Flot plot object.
     */
    var plotRef = $('#chart').plot([],
        {
          legend: {
            show: false
          },
          grid: {
            hoverable: true,
            autoHighlight: true,
            mouseActiveRadius: 16,
            clickable: true
          },
          xaxis: {
            ticks: function(axis) {
              var range = axis.max - axis.min;
              // Different possible tick intervals, ranging from a second to
              // about a year
              var scaleFactors = [1, 2, 3, 5, 10, 15, 20, 30, 45, 60, 120,
                                  240, 300, 900, 1200, 1800, 3600, 7200,
                                  10800, 14400, 18000, 21600, 43200, 86400,
                                  604800, 2592000, 5184000, 10368000, 
                                  15552000, 31536000];
              var MAX_TICKS = 5;
              var i = 0;
              while (range / scaleFactors[i] > MAX_TICKS &&
                  i < scaleFactors.length) {
                i++;
              }
              var scaleFactor = scaleFactors[i];
              var cur = scaleFactor * Math.ceil(axis.min / scaleFactor);
              var ticks = [];
              do {
                var tickDate = new Date(cur * 1000);
                var formattedTime = tickDate.toString();
                if (scaleFactor >= 24 * 60 * 60) {
                  formattedTime = tickDate.toDateString();
                } else {
                  // FUTURE: Find a way to make a string with only the hour or minute
                  formattedTime = tickDate.toDateString() + '<br \\>' +
                    tickDate.toTimeString();
                }
                ticks.push([cur, formattedTime]);
                cur += scaleFactor;
              } while (cur < axis.max);
              return ticks;
            }
          },
          crosshair: {
            mode: 'xy'
          },
          zoom: {
            interactive: true
          },
          pan: {
            interactive: true,
            frameRate: 60
          }
        }).data('plot');

    // Redraw the plot when traces are modified.
    new ArrayObserver(traces).open(function(slices) {
      plotRef.setData(traces);
      var options = plotRef.getOptions();
      var cleanAxes = function(axis) {
        axis.max = null;
        axis.min = null;
      };
      options.xaxes.forEach(cleanAxes);
      options.yaxes.forEach(cleanAxes);
      plotRef.setupGrid();
      plotRef.draw();

      var data = plotRef.getData();
      console.log(data);
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
      while (queryTable.hasChildNodes()) {
        queryTable.removeChild(queryTable.lastChild);
      }

      for (var i = 0; i < queryInfo.params.length; i++) {
        var column = document.createElement('td');

        var longest = Math.max.apply(null, queryInfo.params[i].map(function(p) {
          return p.length;
        }));
        var minWidth = 0.75 * longest + 0.5;

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

        for (var j = 1; j < queryInfo.params[i].length; j++) {
          var option = document.createElement('option');
          option.value = queryInfo.params[i][j];
          option.innerText = queryInfo.params[i][j].length > 0 ?
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
    var dataSet = 'skps';
    var tileNum = [-1];
    var scale = 0;
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
  if (document.readyState != 'loading') {
    onLoad();
  } else {
    this.addEventListener('load', onLoad);
  }

  return {
    traces: traces,
    queryInfo: queryInfo
  };

}());
