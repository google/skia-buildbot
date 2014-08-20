/**
 * The communication between parts of the code will be done by using Object.observe
 * on common data structures.
 *
 * The data structures are 'traces__', 'queryInfo__', 'commitData__', 'dataset__':
 *
 *   traces__
 *     - A list of objects that can be passed directly to Flot for display.
 *   queryInfo__
 *     - A list of all the keys and the parameters the user can search by.
 *   commitData__
 *     - A list of commits for the current set of tiles.
 *   dataset__
 *     - The current scale and range of tiles we are working with.
 *
 * There are three objects that interact with those data structures:
 *
 * Plot
 *   - Handles plotting the data in traces via Flot.
 * Query
 *   - Allows the user to select which traces to display.
 * Navigation
 *   - Allows the user to move among tiles, change scale, etc.
 *
 */
var skiaperf = (function() {
  "use strict";

  /**
   * Stores the trace data.
   * Formatted so it can be directly fed into Flot generate the plot,
   * Plot observes traces__, and Navigation can make changes to traces__.
   */
  var traces__ = [
      /*
      {
        // All of these keys and values should be exactly what Flot consumes.
        data: [[1, 1.1], [20, 30]],
        label: "key1",
        color: "",
        lines: {
          show: false
        },
        _params: {
          os: "Android",
          ...
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
   * Navigation modifies commitData__.
   * Plot reads it.
   */
  var commitData__ = [];

  /**
   * The data needed by Query to build a UI for filtering traces.
   *
   * Query observes fields in queryInfo__.
   * Navigation can modify queryInfo__.
   *
   * Note that queryInfo_ is passed as a parameter to Query, which means that
   * Query will see changes to queryInfo__ fields. which is intended.
   */
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
   * The results for the trybot.
   */
  var trybotResults__ = {
      /*
       'trace:key': 13.234  // The value of the trybot result.
      */
  };

  /**
   * The current scale, set of tiles, and tick marks for the data we are viewing.
   *
   * Navigation can change this.
   */
  var dataset__ = {
    scale: 0,
    tiles: [-1],
    ticks: [],
    skps: []     // The indices where SKPs were regenerated.
  };


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
   * Plot observes traces__.
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
     * Stores the keys of the currently selected lines, used in the drawSeries
     * hook to highlight that line.
     */
    this.curHighlightedLines = [];

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
   * Clears out UI elements back to blank.
   */
  Plot.prototype.clear = function() {
    $$$('#note').classList.add("hidden");
    this.curHighlightedLines = [];
    this.plotLabel.value = "";
  }


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
   * Highlight every line in curHighlightedLines by increasing its line width.
   */
  Plot.prototype.drawHighlightedLine = function(plot, canvascontext, series) {
    if (!series.lines) {
      series.lines = {};
    }
    if (!series.points) {
      series.points = {};
    }
    if (-1 != this.curHighlightedLines.indexOf(series.label)) {
      series.lines.lineWidth = 5;
      series.points.show = true;
    } else {
      series.lines.lineWidth = 1;
      series.points.show = false;
    }
  };


  /**
   * addParamToNote adds a single key, value parameter pair to the note card.
   */
  Plot.prototype.addParamToNote = function(parent, key, value) {
    var node = $$$('#note-param-template').content.cloneNode(true);
    $$$('.key', node).textContent = key;
    var v = $$$('.value', node);
    v.textContent = value;
    v.dataset.key = key;
    v.dataset.value = value;
    parent.appendChild(node);
  }

  /**
   * getMarkings is called by Flot's grid.markings.
   *
   * Draw bands to indicate updates to the SKP files.
   */
  Plot.getMarkings = function(axes) {
    // Create a new array surrounded with 0 and the last commit index.
    // I.e.  [12, 25] -> [0, 12, 25, 127]
    var all = [0].concat(dataset__.skps, [commitData__.length-1]);
    var ret = [];
    // Add in a gray band at every other pair of points.
    for (var i = 2, len = all.length; i < len; i+=2) {
      ret.push({ xaxis: {from: all[i], to: all[i-1]}, color: '#eeeeee'});
    }
    return ret
  };

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
            markings: Plot.getMarkings
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
        if (item) {
          $$$('#plot-value').value = item.datapoint[1].toPrecision(5);
        } else {
          $$$('#plot-value').value = "";
        }
        $$$('#note .group-only').classList.add("hidden");
        if (traces__.length > 0 && pos.x && pos.y) {
          // Find the trace with the closest perpendicular distance, and
          // highlight the trace if it's within N units of pos.
          var closestTraceIndex = 0;
          var closestDistance = Number.POSITIVE_INFINITY;
          for (var i = 0; i < traces__.length; i++) {
            var curTraceData = traces__[i].data;
            if (curTraceData.length <= 1) {
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

          var lastHighlightedLines = plot_.curHighlightedLines.slice(0);

          var yaxis = plot_.plotRef.getAxes().yaxis;
          var maxDist = 0.15 * (yaxis.max - yaxis.min);
          if (closestDistance < maxDist) {
            // Highlight that trace.
            plot_.plotLabel.value = traces__[closestTraceIndex].label;
            plot_.curHighlightedLines = [traces__[closestTraceIndex].label];
          }
          lastHighlightedLines.sort();
          plot_.curHighlightedLines.sort();
          if (!sk.array.equal(lastHighlightedLines, plot_.curHighlightedLines)) {
            plot_.plotRef.draw();
          }
        }
      };
    }()));

    jQuery('#chart').bind('plotclick', function(evt, pos, item) {
      if (!item) {
        return;
      }
      $$$('#note .group-only').classList.add("hidden");
      $$$('#note').dataset.key = item.series.label;

      // First, find the range of CLs we are interested in.
      var thisCommitOffset = item.datapoint[0];
      var thisCommit = commitData__[thisCommitOffset].hash;
      var query = '?begin=' + thisCommit;
      if (item.dataIndex > 0) {
        var previousCommitOffset = item.series.data[item.dataIndex-1][0]
        var previousCommit = commitData__[previousCommitOffset].hash;
        query = '?begin=' + previousCommit + '&end=' + thisCommit;
      }
      // Fill in commit info from the server.
      sk.get('/commits/' + query).then(function(html){
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
      // Enable selecting a group of lines by parameter values.
      $$('#note .value').forEach(function(e){
        e.addEventListener('click', function(e) {
          $$$('#note .group-only').classList.remove("hidden");
          // Highlight every line that matches this parameters key,value.
          plot_.curHighlightedLines = [];
          var pkey = this.dataset.key;
          var pvalue = this.dataset.value;
          traces__.forEach(function(tr) {
            if (tr._params[pkey] == pvalue) {
              plot_.curHighlightedLines.push(tr.label);
            }
          });
          plot_.plotRef.draw();
          e.preventDefault();
        });
      });
      $$$('#note').classList.remove("hidden");
    });

    // Remove all other traces when this is clicked.
    $$$('#note .make-solo').addEventListener('click', function(e) {
      var key = $$$('#note').dataset.key;
      if (key) {
        var trace = null;
        var len = traces__.length;
        for (var i=0; i<len; i++) {
          if (traces__[i].label == key) {
            trace = traces__[i];
          }
        }
        if (trace) {
          traces__.splice(0, len, trace);
        }
      }
      e.preventDefault();
    });

    // Remove all traces that aren't currently highlighted.
    $$$('#note .group-only').addEventListener('click', function() {
      for (var i = traces__.length-1; i >= 0; i--) {
        if (-1 == plot_.curHighlightedLines.indexOf(traces__[i].label)) {
          traces__.splice(i, 1);
        }
      }
    });

    // Remove this trace.
    $$$('#note .remove').addEventListener('click', function() {
      var key = $$$('#note').dataset.key;
      for (var i = 0, len = traces__.length; i < len; i++) {
        if (key == traces__[i].label) {
          traces__.splice(i, 1);
          break;
        }
      }
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

    // Redraw the plot when traces__ are modified.
    //
    // FIXME: Our polyfill doesn't have Array.observe, so this fails on FireFox.
    Array.observe(traces__, function(splices) {
      console.log(splices);
      plot_.plotRef.setData(traces__);
      if (dataset__.ticks.length) {
        plot_.plotRef.getOptions().xaxes[0]["ticks"] = dataset__.ticks;
      }
      plot_.plotRef.setupGrid();
      plot_.plotRef.draw();
    });


    // Redraw the plot when dataset__ is modified, in particular the ticks.
    //
    Object.observe(dataset__, function(splices) {
      plot_.plotRef.getOptions().xaxes[0]["ticks"] = dataset__.ticks;
      plot_.plotRef.setupGrid();
      plot_.plotRef.draw();
    });



    // Update annotation points
    Object.observe(commitData__, function() {
      console.log(Object.keys(commitData__));
      var timestamps = Object.keys(commitData__).map(function(e) {
        return parseInt(e);
      });
      console.log(timestamps);
      var startTime = Math.min.apply(null, timestamps);
      var endTime = Math.max.apply(null, timestamps);
      sk.get('annotations/?start=' + startTime + '&end=' + endTime).then(JSON.parse).then(function(json){
        var commitToTimestamp = {};
        Object.keys(commitData__).forEach(function(timestamp) {
          if (commitData__[timestamp]['hash']) {
            commitToTimestamp[commitData__[timestamp]['hash']] = timestamp;
          }
        });
        Object.keys(json).forEach(function(hash) {
          if (commitToTimestamp[hash]) {
            plot_.annotations[commitToTimestamp[hash]] = json[hash];
          } else {
            console.log('WARNING: Annotation taken for commit not stored in' +
                ' commitData__');
          }
        });
        // Redraw to get the new lines
        plot_.plotRef.draw();
      });
      req.send();
    });

  }



  /**
   * Manages the tile scale and index that the user can query over.
   */
  function Navigation(query, plot) {
    // Keep tracking if we are still loading the page the first time.
    this.loading_ = true;

    this.query_ = query;

    this.plot_ = plot;
  };


  /**
   * Adds Traces that match the given query params.
   *
   * q is a URL query to be appended to /query/<scale>/<tiles>/traces/.
   * The matching traces are returned and added to the plot.
   */
  Navigation.prototype.addTraces = function(q) {
    var navigation = this;
    sk.get("/query/0/-1/traces/?" + q).then(JSON.parse).then(function(json){
      json["traces"].forEach(function(t) {
        traces__.push(t);
      });
    }).then(function(){
      navigation.loading_ = false;
    }).catch(notifyUser);
  }


  /**
   * Load shortcuts if any are present in the URL.
   */
  Navigation.prototype.loadShortcut = function() {
    if (window.location.hash.length > 2) {
      this.addTraces("__shortcut=" + window.location.hash.substr(1))
    }
  }

  Navigation.prototype.attach = function() {
    var navigation_ = this;

    $$$('#add-lines').addEventListener('click', function() {
      navigation_.addTraces(navigation_.query_.selectionsAsQuery())
    });

    $$$('#shortcut').addEventListener('click', function() {
      // Package up the current state and stuff it into the database.
      var state = {
        scale: 0,
        tiles: [-1],
        keys: traces__.map(function(t) { return t.label; })
        // Maybe preserve selections also?
      };
      sk.post("/shortcuts/", JSON.stringify(state)).then(JSON.parse).then(function(json) {
        // Set the shortcut in the hash.
        window.history.pushState(null, "", "#" + json.id);
      });
    });

    $$$('#nuke-plot').addEventListener('click', function(e) {
      traces__.splice(0, traces__.length);
      navigation_.plot_.clear();
      navigation_.query_.clear();
    });

    Array.observe(traces__, function() {
      // Any changes to the traces after we're fully loaded should clear the
      // shortcut from the hash.
      if (navigation_.loading_ == false) {
        window.history.pushState(null, "", "#");
      }
    });

    sk.get('/tiles/0/-1/').then(JSON.parse).then(function(json){
      queryInfo__.paramSet = json.paramset;
      dataset__.scale = json.scale;
      dataset__.tiles = json.tiles;
      dataset__.ticks = json.ticks;
      dataset__.skps = json.skps;
      commitData__ = json.commits;
      queryInfo__.change.counter += 1;
    });
  };


  /**
   * Gets the Object.observe events delivered, only in the case we are
   * using a polyfill.
   */
  function microtasks() {
    setTimeout(microtasks, 125);
  }


  function onLoad() {
    var query = new sk.Query(queryInfo__);
    query.attach();

    var plot = new Plot();
    plot.attach();

    var navigation = new Navigation(query, plot);
    navigation.attach();


    microtasks();

    navigation.loadShortcut();
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
