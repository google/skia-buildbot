/**
 * The communication between parts of the code will be done by using Object.observe
 * on common data structures.
 *
 * The data structures are 'traces', 'queryInfo', 'commitData', 'dataset':
 *
 *   traces
 *     - A list of objects that can be passed directly to Flot for display.
 *   queryInfo
 *     - A list of all the keys and the parameters the user can search by.
 *   commitData
 *     - A list of commits for the current set of tiles.
 *   dataset
 *     - The current scale and range of tiles we are working with.
 *
 * There are three objects that interact with those data structures:
 *
 * Plot
 *   - Handles plotting the data in traces via Flot.
 * Query
 *   - Allows the user to select which traces to display.
 * Dataset
 *   - Allows the user to move among tiles, change scale, etc.
 *
 */
var skiaperf = (function() {
  "use strict";

  /**
   * Stores the trace data.
   * Formatted so it can be directly fed into Flot generate the plot,
   * Plot observes traces, and Query can make changes to traces.
   */
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
      ...
      */
    ];

  /**
   * Contains all the information about each commit.
   *
   * A list of commit objects where the offset of the commit in the list
   * matches the offset of the value in the trace.
   *
   * Dataset modifies commitData.
   * Plot reads it.
   */
  var commitData = [];

  /**
   * Stores the different parameters that can be used to specify a trace.
   * The {@code params} field contains an array of arrays, each array
   * representing a single parameter that can be set, with the first element of
   * the array being the human readable name for it, and each followin element
   * a different possibility of what to set it to.
   * The {@code trybotResults} fields contains a dictionary of trace keys,
   * whose values are the trybot results for each trace.
   * Query observe queryInfo, and Dataset and Query can modify queryInfo.
   */
  var queryInfo = {
    params: {
      /*
      "benchName": ["desk_gmailthread.skp", "desk_mapsvg.skp" ],
      "timer":     ["wall", "cpu"],
      "arch":      ["arm7", "x86", "x86_64"],
      */
    },
    trybotResults: {
      /*
       'trace:key': 13.234  // The value of the trybot result.
      */
    }
  };

  /**
   * The current scale, set of tiles, and tick marks for the data we are viewing.
   *
   * Dataset can change this.
   * Query observes this and updates traces and queryInfo.params when it changes.
   */
  var dataset = {
    scale: 0,
    tiles: [-1],
    ticks: []
  };

  // Query watches queryChange.
  // Dataset can change queryChange.
  //
  // queryChange is used because Observe-js has trouble dealing with the large
  // array changes that happen when Dataset swaps queryInfo data.
  var queryChange = { counter: 0 };


  /******************************************
   * Utility functions used across this file.
   ******************************************/

  /**
   * $$ returns a real JS array of DOM elements that match the CSS query selector.
   *
   * A shortcut for jQuery-like $ behavior.
   **/
  function $$(query, ele) {
    if (!ele) {
      ele = document;
    }
    return Array.prototype.map.call(ele.querySelectorAll(query), function(e) { return e; });
  }


  /**
   * $$$ returns the DOM element that match the CSS query selector.
   *
   * A shortcut for document.querySelector.
   **/
  function $$$(query, ele) {
    if (!ele) {
      ele = document;
    }
    return ele.querySelector(query);
  }

  /**
   * clearChildren removes all children of the passed in node.
   */
  function clearChildren(ele) {
    while (ele.firstChild) {
      ele.removeChild(ele.firstChild);
    }
  }


  // escapeNewlines replaces newlines with <br />'s
  function escapeNewlines(str) {
    return (str + '').replace(/\n/g, '<br />');
  }

  // Returns a Promise that uses XMLHttpRequest to make a request to the given URL.
  function get(url) {
    // Return a new promise.
    return new Promise(function(resolve, reject) {
      // Do the usual XHR stuff
      var req = new XMLHttpRequest();
      req.open('GET', url);

      req.onload = function() {
        // This is called even on 404 etc
        // so check the status
        if (req.status == 200) {
          // Resolve the promise with the response text
          resolve(req.response);
        } else {
          // Otherwise reject with the status text
          // which will hopefully be a meaningful error
          reject(Error(req.statusText));
        }
      };

      // Handle network errors
      req.onerror = function() {
        reject(Error("Network Error"));
      };

      // Make the request
      req.send();
    });
  }


  /**
   * Converts from a POSIX timestamp to a truncated RFC timestamp that
   * datetime controls can read.
   */
  function toRFC(timestamp) {
    return new Date(timestamp * 1000).toISOString().slice(0, -1);
  }

  /**
   * Notifies the user.
   */
  function notifyUser(err) {
    alert(err);
  }


  /**
   * Sets up the callbacks related to the plot.
   * Plot observes traces.
   */
  function Plot() {
    /**
     * Stores the annotations currently visible on the plot. The hash is used
     * as a key to either an object like:
     *
     * {
     *   id: 7,
     *   notes: "Something happened here",
     *   author: "bensong",
     *   type: 0
     * }
     * or null.
     */
    this.annotations = {};

    /**
     * Used to determine if the scale of the y-axis of the plot.
     * If it's true, a logarithmic scale will be used. If false, a linear
     * scale will be used.
     */
    this.isLogPlot = false;

    /**
     * Stores the name of the currently selected line, used in the drawSeries
     * hook to highlight that line.
     */
    this.curHighlightedLine = null;

    /**
     * Reference to the underlying Flot data.
     */
    this.plotRef = null;

    /**
     * The element is used to display commit and annotation info.
     */
    this.note = null;

    /**
     * The element displays the current trace we're hovering over.
     */
    this.plotLabel = null;
  };


  /**
   * Draws vertical lines that pass through the times of the loaded annotations.
   * Declared here so it can be used in plotRef's initialization.
   */
  Plot.prototype.drawAnnotations = function(plot, context) {
    var yaxes = plot.getAxes().yaxis;
    var offsets = plot.getPlotOffset();
    Object.keys(this.annotations).forEach(function(timestamp) {
      var lineStart = plot.p2c({'x': timestamp, 'y': yaxes.max});
      var lineEnd = plot.p2c({'x': timestamp, 'y': yaxes.min});
      context.save();
      var maxLevel = -1;
      this.annotations[timestamp].forEach(function(annotation) {
        if (annotation.type > maxLevel) {
          maxLevel = annotation.type;
        }
      });
      switch (maxLevel) {
        case 1:
          context.strokeStyle = 'dark yellow';
          break;
        case 2:
          context.strokeStyle = 'red';
          break;
        default:
          context.strokeStyle = 'grey';
      }
      context.beginPath();
      context.moveTo(lineStart.left + offsets.left,
          lineStart.top + offsets.top);
      context.lineTo(lineEnd.left + offsets.left, lineEnd.top + offsets.top);
      context.closePath();
      context.stroke();
      context.restore();
    });
  };


  /**
   * Hook for drawSeries.
   * If curHighlightedLine is not null, drawHighlightedLine highlights
   * the line by increasing its line width.
   */
  Plot.prototype.drawHighlightedLine = function(plot, canvascontext, series) {
    if (!series.lines) {
      series.lines = {};
    }
    series.lines.lineWidth = series.label == this.curHighlightedLine ? 5 : 1;

    if (!series.points) {
      series.points = {};
    }
    series.points.show = (series.label == this.curHighlightedLine);
  };


  /**
   * addParamToNote adds a single key, value parameter pair to the note card.
   */
  Plot.prototype.addParamToNote = function(parent, key, value) {
    var node = $$$('#note-param').content.cloneNode(true);
    $$$('.key', node).textContent = key;
    $$$('.value', node).textContent = value;
    parent.appendChild(node);
  }

  /**
   * attach hooks up all the controls to the Plot instance.
   */
  Plot.prototype.attach = function() {
    var plot_ = this;

    this.note = $$$('#note');
    this.plotLabel = $$$('#plot-label');


    /**
     * Reference to the underlying Flot plot object.
     */
    this.plotRef = jQuery('#chart').plot([],
        {
          legend: {
            show: false
          },
          grid: {
            hoverable: true,
            autoHighlight: true,
            mouseActiveRadius: 16,
            clickable: true,
          },
          xaxis: {
            ticks: [],
            zoomRange: false,
            panRange: false,
          },
          yaxis: {
            transform: function(v) { return plot_.isLogPlot? Math.log(v) : v; },
            inverseTransform: function(v) { return plot_.isLogPlot? Math.exp(v) : v; }
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
          },
          hooks: {
            draw: [plot_.drawAnnotations.bind(plot_)],
            drawSeries: [plot_.drawHighlightedLine.bind(plot_)]
          }
        }).data('plot');


    jQuery('#chart').bind('plothover', (function() {
      return function(evt, pos, item) {
        if (traces.length > 0 && pos.x && pos.y) {
          // Find the trace with the closest perpendicular distance, and
          // highlight the trace if it's within N units of pos.
          var closestTraceIndex = 0;
          var closestDistance = Number.POSITIVE_INFINITY;
          for (var i = 0; i < traces.length; i++) {
            var curTraceData = traces[i].data;
            if (curTraceData.length <= 1 || !traces[i].lines.show) {
              continue;
            }
            var j = 1;
            // Find the pair of datapoints where
            // data[j-1][0] < pos.x < data[j][0].
            // We want j to also never equal curTraceData.length, so we limit
            // it to curTraceData.length - 1.
            while(j < curTraceData.length - 1 && curTraceData[j][0] < pos.x) {
              j++;
            }
            // Make sure j - 1 >= 0.
            if (j == 0) {
              j ++;
            }
            var xDelta = curTraceData[j][0] - curTraceData[j - 1][0];
            var yDelta = curTraceData[j][1] - curTraceData[j - 1][1];
            var lenDelta = Math.sqrt(xDelta*xDelta + yDelta*yDelta);
            var perpDist = Math.abs(((pos.x - curTraceData[j][0]) * yDelta -
                  (pos.y - curTraceData[j][1]) * xDelta) / lenDelta);
            if (perpDist < closestDistance) {
              closestTraceIndex = i;
              closestDistance = perpDist;
            }
          }

          var lastHighlightedLine = plot_.curHighlightedLine;

          var yaxis = plot_.plotRef.getAxes().yaxis;
          var maxDist = 0.15 * (yaxis.max - yaxis.min);
          if (closestDistance < maxDist) {
            // Highlight that trace.
            plot_.plotLabel.value = traces[closestTraceIndex].label;
            plot_.curHighlightedLine = traces[closestTraceIndex].label;
          }
          if (lastHighlightedLine != plot_.curHighlightedLine) {
            plot_.plotRef.draw();
          }
        }
      };
    }()));

    jQuery('#chart').bind('plotclick', function(evt, pos, item) {
      if (!item) {
        return;
      }
      $$$('#note').dataset.key = item.series.label;

      // First, find the range of CLs we are interested in.
      var thisCommitOffset = item.datapoint[0];
      var thisCommit = commitData[thisCommitOffset].hash;
      var query = '?begin=' + thisCommit;
      if (item.dataIndex > 0) {
        var previousCommitOffset = item.series.data[item.dataIndex-1][0]
        var previousCommit = commitData[previousCommitOffset].hash;
        query = '?begin=' + previousCommit + '&end=' + thisCommit;
      }
      // Fill in commit info from the server.
      get('/commits/' + query).then(function(html){
        $$$('#note .commits').innerHTML = html;
      });

      // Add params to the note.
      var parent = $$$('#note .params');
      clearChildren(parent);
      plot_.addParamToNote(parent, 'id', item.series.label);
      var keylist = Object.keys(item.series._params).sort().reverse();
      for (var i = 0; i < keylist.length; i++) {
        var key = keylist[i];
        plot_.addParamToNote(parent, key, item.series._params[key]);
      }

      $$$('#note').classList.remove("hidden");

    });

    $$$('.make-solo').addEventListener('click', function(e) {
      var key = $$$('#note').dataset.key;
      if (key) {
        var trace = null;
        var len = traces.length;
        for (var i=0; i<len; i++) {
          if (traces[i].label == key) {
            trace = traces[i];
          }
        }
        if (trace) {
          traces.splice(0, len, trace);
        }
      }
      e.preventDefault();
    });

    $$$('#reset-axes').addEventListener('click', function(e) {
      var options = plot_.plotRef.getOptions();
      var cleanAxes = function(axis) {
        axis.max = null;
        axis.min = null;
      };
      options.xaxes.forEach(cleanAxes);
      options.yaxes.forEach(cleanAxes);

      plot_.plotRef.setupGrid();
      plot_.plotRef.draw();
    });

    // Redraw the plot when traces are modified.
    //
    // FIXME: Our polyfill doesn't have Array.observe, so this fails on FireFox.
    Array.observe(traces, function(splices) {
      console.log(splices);
      plot_.plotRef.setData(traces);
      if (dataset.ticks.length) {
        plot_.plotRef.getOptions().xaxes[0]["ticks"] = dataset.ticks;
      }
      plot_.plotRef.setupGrid();
      plot_.plotRef.draw();
      plot_.updateEdges();
    });

    // Update annotation points
    Object.observe(commitData, function() {
      console.log(Object.keys(commitData));
      var timestamps = Object.keys(commitData).map(function(e) {
        return parseInt(e);
      });
      console.log(timestamps);
      var startTime = Math.min.apply(null, timestamps);
      var endTime = Math.max.apply(null, timestamps);
      get('annotations/?start=' + startTime + '&end=' + endTime).then(JSON.parse).then(function(json){
        var commitToTimestamp = {};
        Object.keys(commitData).forEach(function(timestamp) {
          if (commitData[timestamp]['hash']) {
            commitToTimestamp[commitData[timestamp]['hash']] = timestamp;
          }
        });
        Object.keys(json).forEach(function(hash) {
          if (commitToTimestamp[hash]) {
            plot_.annotations[commitToTimestamp[hash]] = json[hash];
          } else {
            console.log('WARNING: Annotation taken for commit not stored in' +
                ' commitData');
          }
        });
        // Redraw to get the new lines
        plot_.plotRef.draw();
      });
      req.send();
    });

    $$$('#nuke-plot').addEventListener('click', function(e) {
      traces.splice(0, traces.length);
      $$$('#note').classList.add("hidden");
      $$$('#query-text').textContent = '';
      plot_.plotLabel.value = "";
      plot_.curHighlightedLine = "";
    });
  }


  /**
   * Sets up the event handlers related to the query controls in the interface.
   * The callbacks in this function use and observe {@code queryInfo},
   * and modifies {@code traces}. Takes the object {@code Dataset} creates
   * as input.
   */
  function Query() {
  };

  // attach hooks up all the controls that Query uses.
  Query.prototype.attach = function() {

    var query_ = this;

    Object.observe(queryChange, this.onParamChange);

    // Add handlers to the query controls.
    $$$('#add-lines').addEventListener('click', function() {
      get('/query/0/-1/traces/?' + query_.selectionsAsQuery()).then(JSON.parse).then(function(json) {
        json["traces"].forEach(function(t) {
          t["lines"] = { show:true };
          traces.push(t);
        });
      }).catch(notifyUser);
    });

    $$$('#inputs').addEventListener('change', function(e) {
      get('/query/0/-1/?' + query_.selectionsAsQuery()).then(JSON.parse).then(function(json) {
        $$$('#query-text').innerHTML = json["matches"] + ' lines selected<br />';
      });
    });

    // TODO add observer on dataset and update the current traces if any are displayed.
    get('/tiles/0/-1/').then(JSON.parse).then(function(json){
      queryInfo.params = json.paramset;
      dataset.scale= json.scale;
      dataset.tiles = json.tiles;
      dataset.ticks = json.ticks;
      commitData = json.commits;
      queryChange.counter += 1;
    });

    $$$('#more-inputs').addEventListener('click', function(e) {
      $$$('#more').classList.toggle('hidden');
    });

    $$$('#clear-selections').addEventListener('click', function(e) {
      // Clear the param selections.
      $$('option:checked').forEach(function(elem) {
        elem.selected = false;
      });
      $$$('#query-text').textContent = '';
    });

  }

  Query.prototype.selectionsAsQuery = function() {
    var sel = [];
    var num = Object.keys(queryInfo.params).length;
    for(var i = 0; i < num; i++) {
      var key = $$$('#select_' + i).name
        $$('#select_' + i + ' option:checked').forEach(function(ele) {
          sel.push(encodeURIComponent(key) + '=' + encodeURIComponent(ele.value));
        });
    }
    return sel.join('&')
  };


  /**
   * Syncs the DOM to match the current state of queryInfo.
   * It currently removes all the existing elements and then
   * generates a new set that matches the queryInfo data.
   */
  Query.prototype.onParamChange = function() {
    console.log('onParamChange() triggered');
    var queryDiv = $$$('#inputs');
    var detailsDiv= $$$('#inputs #more');
    // Remove all old nodes.
    $$('#inputs .query-node').forEach(function(ele) {
      ele.parentNode.removeChild(ele)
    });

    var whitelist = ['test', 'os', 'source_type', 'scale', 'extra_config', 'config', 'arch'];
    var keylist = Object.keys(queryInfo.params).sort().reverse();

    for (var i = 0; i < keylist.length; i++) {
      var node = $$$('#query-select').content.cloneNode(true);
      var key = keylist[i];

      $$$('h4', node).textContent = key;

      var select = $$$('select', node);
      select.id = 'select_' + i;
      select.name = key;

      var options = queryInfo.params[key].sort();
      options.forEach(function(op) {
        var option = document.createElement('option');
        option.value = op;
        option.textContent = op.length > 0 ?  op : '(none)';
        select.appendChild(option);
      });

      if (whitelist.indexOf(key) == -1) {
        detailsDiv.insertBefore(node, detailsDiv.firstElementChild);
      } else {
        queryDiv.insertBefore(node, queryDiv.firstElementChild);
      }
    }
  }


  /**
   * Manages the set of keys the user can query over.
   * Returns an object containing a reference to requestTiles and
   * tileNums
   */
  function Dataset() {
  };


  Dataset.prototype.attach = function() {
    // TODO(jcgregorio) add in tile moving controls and monitor them from here.
  };


  /**
   * Gets the Object.observe events delivered, only in the case we are
   * using a polyfill.
   */
  function microtasks() {
    setTimeout(microtasks, 125);
  }


  function onLoad() {
    var dataset = new Dataset();
    dataset.attach();

    var query = new Query();
    query.attach();

    var plot = new Plot();
    plot.attach();

    microtasks();
    Object.observe(dataset, function() {
      console.log('dataset changed!', dataset);
    });
  }

  // If loaded via HTML Imports then DOMContentLoaded will be long done.
  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

  return {
    $$: $$,
    $$$: $$$,
  };
}());
