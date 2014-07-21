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
 *   commitData
 *     - A dictionary from the timestamp of a commit (converted to a string)
 *       to an object containing all the data about that commit.
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

  /**
   * Contains all the information about each commit.
   * It uses an {@code Object} as a dictionary, where the key is the time of
   * the commit. Dataset modifies commitData, Plot sometimes reads it.
   */
  var commitData = {};

  // Query uses queryInfo.
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
  // Query watches queryChange.
  // Dataset can change queryChange.
  var queryChange = { counter: 0 };
  // queryChange is used because Observe-js has trouble dealing with the large
  // array changes that happen when Dataset swaps queryInfo data.

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


  /**
   * Renders the legend and keeps it in sync with the visible traces.
   * Legend watches traces, and changes the elements inside of #legend-table
   * to match traces. Currently it removes all the elements and regenerates
   * them all from a template, but this seems to work well enough for the
   * time being.
   */
  function Legend() {
    var legendTemplate = $$$('#legend-entry');
    new ArrayObserver(traces).open(function(slices) {
      // FUTURE: Make more efficient if necessary
      // Clean legend element
      var legendTable = $$$('#legend table tbody');
      while (legendTable.hasChildNodes()) {
        legendTable.removeChild(legendTable.lastChild);
      }
      traces.forEach(function(trace) {
        // Add a new trace to the legend.
        var traceName = trace.label;
        var newLegendEntry = legendTemplate.content.cloneNode(true);
        var checkbox = $$$('input', newLegendEntry);
        checkbox.checked = !!trace.lines.show;
        checkbox.id = traceName;
        var label = $$$('label', newLegendEntry);
        label.setAttribute('for', traceName);
        label.innerText = traceName;
        $$$('a', newLegendEntry).id = 'remove_' + traceName;

        legendTable.appendChild(newLegendEntry);
      });
    });

    $$$('#legend tbody').addEventListener('click', function(e) {
      if (e.target.nodeName == 'INPUT') {
        for (var i = 0; i < traces.length; i++) {
          if (traces[i].label == e.target.id) {
            traces[i] = {
              color: traces[i].color,
              data: traces[i].data,
              label: traces[i].label,
              lines: {
                show: e.target.checked
              }
            };
            break;
          }
        }
        if (i == traces.length) {
          console.log('Error: legend contains invalid trace');
        }
      } else if (e.target.nodeName == 'A') {
        var targetName = e.target.id.slice('remove_'.length);
        for (var i = 0; i < traces.length; i++) {
          if (traces[i].label == targetName) {
            traces.splice(i, 1);
            break;
          }
        }
        if (i == traces.length) {
          console.log('Error: legend contains invalid trace');
        }
      }
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
    new ObjectObserver(queryChange).open(onParamChange);
  }


  /**
   * Manages the set of keys the user can query over.
   */
  function Dataset() {
    // These describe the current "window" of data we're looking at.
    var dataSet = 'skps';
    var tileNums = [-1];
    var scale = 0;

    /**
     * Helps make requests for a set of tiles.
     * Makes a XMLHttpRequest for using the data in {@code dataSet}, 
     * {@code tileNums}, and {@code scale}, using the data in moreParams
     * as requery query parameters. Calls finished with the data or
     * an empty object when finished.
     */
    function requestTiles(finished, moreParams) {
      var onloaderror = function() {
        finished({});
      };

      var onloadfinish = function() {
        document.body.classList.remove('waiting');
      };

      var request = new XMLHttpRequest();

      var onjsonload = function() {
        if (request.response && request.status == 200) {
          if (request.responseType == 'json') {
            finished(request.response);
          } else {
            finished(JSON.parse(request.response));
          }
        }
      };

      var params = '';
       if(moreParams) {
        Object.keys(moreParams).forEach(function(key) {
          params += encodeURIComponent(key) + '=' + 
              encodeURI(moreParams[key]) + '&';
        });
      }

      request.open('GET', ['tiles', dataSet, scale, tileNums.join(',')].
            join('/') + '?' + params);
      document.body.classList.add('waiting');
      request.addEventListener('load', onjsonload);
      request.addEventListener('error', onloaderror);
      request.addEventListener('loadend', onloadfinish);
      request.send();
    }



    /**
     * Updates queryInfo.params and queryInfo.allKeys.
     * It requests the tile key data from all the tiles in tileNum, and
     * sets the queryInfo data to union of each tile's queryInfo data.
     */
    var update = function() {
      var totalParams = [];
      var newNames = {};
      var processJSON = function(json) {
        console.log('Dataset update start');
        if (json['tiles']) {
          json['tiles'].forEach(function(tile) {
            if (tile['params']) {
              // NOTE: Replace with hash map-based thing if not fast enough
              for (var i = 0; i < tile['params'].length; i++) {
                if (!totalParams[i]) {
                  totalParams[i] = [];
                  totalParams[i][0] = tile['params'][i][0];
                }
                for (var j = 1; j < tile['params'][i].length; j++) {
                  if (totalParams[i].indexOf(tile['params'][i][j]) == -1) {
                    totalParams[i].push(tile['params'][i][j]);
                  }
                }
              }
            }
            if (tile['names']) {
              tile['names'].forEach(function(name) {
                newNames[name] = true;
              });
            }
            if (tile['commits']) {
              tile['commits'].forEach(function(commit) {
                commitData[parseInt(commit['commit_time']) + ''] = commit;
              });
            }
          });
        }

        while (queryInfo.allKeys.length > 0) { queryInfo.allKeys.pop(); }
        var newNameList = Object.keys(newNames);
        for (var i = 0; i < newNameList.length; i++) {
          queryInfo.allKeys.push(newNameList[i]);
        }
        for (var i = 0; i < totalParams.length; i++) {
          // Sort params, but keep the name of the column in the first slot
          var tmp = totalParams[i].splice(0, 1)[0];
          totalParams[i].sort();
          totalParams[i].splice(0, 0, tmp);
        }
        while (queryInfo.params.length > 0) { queryInfo.params.pop(); }
        for (var i = 0; i < totalParams.length; i++) {
          queryInfo.params.push(totalParams[i]);
        }
        console.log('Dataset update end');
        queryChange.counter++;
      };

      requestTiles(processJSON, '');
    };


    // Sets up the event binding on the radio controls.
    $$('input[name=schema-type]').forEach(function(e) {
      if (e.value == dataSet) { e.checked = true; }
      e.addEventListener('change', function() {
        while (traces.length) { traces.pop(); }
        dataSet = this.value;
        console.log(dataSet);
        update();
      });
    });

    update();
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
  }

  // If loaded via HTML Imports then DOMContentLoaded will be long done.
  if (document.readyState != 'loading') {
    onLoad();
  } else {
    this.addEventListener('load', onLoad);
  }

  return {
    traces: traces,
    queryInfo: queryInfo,
    commitData: commitData
  };

}());
