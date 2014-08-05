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
  "use strict";

  /**
   * Stores the trace data.
   * Formatted so it can be directly fed into Flot generate the plot,
   * Plot and Legend observe traces, and Query and Legend can make changes to
   * traces.
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
   * Stores the color of each trace in rgb format.
   * Plot can change plotColors, and legend watches plotColors.
   */
  var plotColors = {};


  /**
   * Contains all the information about each commit.
   * It uses an {@code Object} as a dictionary, where the key is the time of
   * the commit. Dataset modifies commitData, Plot sometimes reads it.
   */
  var commitData = {};

  /**
   * Stores the different parameters that can be used to specify a trace.
   * The {@code allKeys} field contains an array of strings representing each
   * possible trace.
   * The {@code params} field contains an array of arrays, each array 
   * representing a single parameter that can be set, with the first element of
   * the array being the human readable name for it, and each followin element
   * a different possibility of what to set it to.
   * The {@code trybotResults} fields contains a dictionary of trace keys,
   * whose values are the trybot results for each trace.
   * Query observe queryInfo, and Dataset and Query can modify queryInfo.
   */
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
    ],
    trybotResults: {
      /*
       'trace:key': 13.234  // The value of the trybot result.
      */
    }
  };
  // Query watches queryChange.
  // Dataset can change queryChange.
  var queryChange = { counter: 0 };
  // queryChange is used because Observe-js has trouble dealing with the large
  // array changes that happen when Dataset swaps queryInfo data.

  function $$(query, ele) {
    if (!ele) {
      ele = document;
    }
    return Array.prototype.map.call(ele.querySelectorAll(query), function(e) { return e; });
  }


  function $$$(query, ele) {
    if (!ele) {
      ele = document;
    }
    return ele.querySelector(query);
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
  function notifyUser(msg) {
    alert(msg);
  }

  /**
   * Sets up the callbacks related to the plot.
   * Plot observes traces.
   */
  function newPlot() {
    /**
     * Stores the edges of the plot to keep the zoom controls in sync.
     * plotEdges is watched to update the zoom and time controls in the UI,
     * and is modified when traces is modified, or the user pans or zooms the
     * plot.
     */
    var plotEdges = [null, null];

    /**
     * Stores the annotations currently visible on the plot. This is updated
     * whenever plotEdges is changed, with a slightly delay to avoid
     * sending too many requests to the server. The hash is used as a key to
     * either an object like
     * {
     *   id: 7,
     *   notes: "Something happened here",
     *   author: "bensong",
     *   type: 0
     * }
     * or null.
     */
    var annotations = {};

    /**
     * Used to determine if the scale of the y-axis of the plot.
     * If it's true, a logarithmic scale will be used. If false, a linear
     * scale will be used.
     */
    var isLogPlot = false;

    /**
     * Stores the name of the currently selected line, used in the drawSeries
     * hook to highlight that line.
     */
    var curHighlightedLine = null;

    /**
     * Draws vertical lines that pass through the times of the loaded annotations.
     * Declared here so it can be used in plotRef's initialization.
     */
    var drawAnnotations = function(plot, context) {
      var yaxes = plot.getAxes().yaxis;
      var offsets = plot.getPlotOffset();
      Object.keys(annotations).forEach(function(timestamp) {
        // Check to see if it's inside the plot before drawing.
        if (timestamp < plotEdges[0] || timestamp > plotEdges[1]) {
          return;
        }
        var lineStart = plot.p2c({'x': timestamp, 'y': yaxes.max});
        var lineEnd = plot.p2c({'x': timestamp, 'y': yaxes.min});
        context.save();
        var maxLevel = -1;
        annotations[timestamp].forEach(function(annotation) {
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
     * Draws trybot lines for traces that have a matching trybot results.
     */
    var drawTrybotResults = function(plot, context) {
      var offsets = plot.getPlotOffset();
      var xaxes = plot.getAxes().xaxis;
      var series = plot.getData();
      series.forEach(function(trace) {
        if (queryInfo.trybotResults[trace.label] && trace.lines.show) {
          var matchingTrybotResult = queryInfo.trybotResults[trace.label];
          context.save();
          var lineStart = plot.p2c({'x': xaxes.min, 'y': matchingTrybotResult});
          var lineEnd = plot.p2c({'x': xaxes.max, 'y': matchingTrybotResult});
          context.strokeStyle = trace.color;
          context.beginPath();
          context.moveTo(lineStart.left + offsets.left,
              lineStart.top + offsets.top);
          context.lineTo(lineEnd.left + offsets.left, lineEnd.top + offsets.top);
          context.closePath();
          context.stroke();
          context.restore();
        }
      });
    };

    /**
     * Gets background markings on SKP version changes.
     */
    var getMarkings = function(axes) {
      if (traces.length <= 0 || !plotEdges[0] || !plotEdges[1]) {
        return [];
      }
      var skpPhrase = 'Update SKP version to ';
      var updates = Object.keys(commitData).map(function(timestamp) {
        return commitData[timestamp];
      }).filter(function(c) {
        return c.commit_time &&
            c.commit_time >= plotEdges[0] && c.commit_time <= plotEdges[1];
      }).filter(function(c) {
        return c.commit_msg && c.commit_msg.indexOf(skpPhrase) >= 0;
      }).map(function(c) {
        return c.commit_time;
      });
      if (updates.length === 0 || updates[0] > plotEdges[0]) {
        updates.unshift(plotEdges[0]);
      }
      if (updates[updates.length - 1] < plotEdges[1]) {
        updates.push(plotEdges[1]);
      }
      var markings = [];
      for (var i = 1; i < updates.length; i++) {
        if (i % 2 === 0) {
          markings.push([updates[i-1], updates[i]]);
        }
      }
      // Alternate white and grey vertical strips.
      var m = markings.map(function(pair) {
        return { xaxis: {from: pair[0], to: pair[1]}, color: '#eeeeee'};
      });
      return m;
    };

    /**
     * Hook for drawSeries.
     * If curHighlightedLine is not null, drawHighlightedLine highlights
     * the line by increasing its line width.
     */
    var drawHighlightedLine = function(plot, canvascontext, series) {
      if (!series.lines) { 
        series.lines = {};
      }
      series.lines.lineWidth = series.label == curHighlightedLine ? 5 : 2;

      if (!series.points) {
        series.points = {};
      }
      series.points.show = (series.label == curHighlightedLine);
    };

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
            clickable: true,
            markings: getMarkings
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
          yaxis: {
            /* zoomRange: false */
            transform: function(v) { return isLogPlot? Math.log(v) : v; },
            inverseTransform: function(v) { return isLogPlot? Math.exp(v) : v; }
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
            draw: [drawAnnotations, drawTrybotResults],
            drawSeries: [drawHighlightedLine]
          }
        }).data('plot');

    /**
     * Updates plotEdges to match the current edges of the plot.
     * This calculates the new plot edges and stores them in plotEdges, using
     * either the {@code xaxis} Flot object or the maximum and minimum of the
     * trace data.
     */
    var updateEdges = function() {
      var data = plotRef.getData();
      var xaxis = plotRef.getOptions().xaxes[0];
      var min = null;
      var max = null;
      if(xaxis.min != null && xaxis.max != null) {
        min = xaxis.min;
        max = xaxis.max;
      } else if(data.length > 0) {
        min = Math.min.apply(null, data.map(function(set) {
          return Math.min.apply(null, set.data.map(function(point) {
            return point[0];
          }));
        }));
        max = Math.max.apply(null, data.map(function(set) {
          return Math.max.apply(null, set.data.map(function(point) {
            return point[0];
          }));
        }));
      }

      plotEdges[0] = min;
      plotEdges[1] = max;
    };

    $$$('#islog').addEventListener('click', function() {
      if($$$('#islog').checked != isLogPlot) {
        isLogPlot = $$$('#islog').checked;
        plotRef.setupGrid();
        plotRef.draw();
      }
    });

    $('#chart').bind('plothover', (function() {
      var lastLabel = null;
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
            if (j == 0) { j ++; }
            var xDelta = curTraceData[j][0] - curTraceData[j - 1][0];
            var yDelta = curTraceData[j][1] - curTraceData[j - 1][1];
            var lenDelta = Math.sqrt(xDelta*xDelta + yDelta*yDelta);
            // assert(lenDelta > 0);
            var perpDist = Math.abs(((pos.x - curTraceData[j][0]) * yDelta -
                (pos.y - curTraceData[j][1]) * xDelta) / lenDelta);
            if (perpDist < closestDistance) {
              closestTraceIndex = i;
              closestDistance = perpDist;
            }
            // Using the perpendicular distance may actually be overkill, come
            // to think of it...
          }

          var lastHighlightedLine = curHighlightedLine;
          // Remove the old label.
          if (lastLabel) { 
            document.body.removeChild(lastLabel); 
            lastLabel = null;
            curHighlightedLine = null;
          }

          var yaxis = plotRef.getAxes().yaxis;
          var maxDist = 0.15 * (yaxis.max - yaxis.min);
          if (closestDistance < maxDist) {
            // Highlight that trace.
            // Okay, trace highlighting doesn't actually work without a plugin.
            // Should I look for a plugin to do this?

            // For now I'll just stick a div with the plot label to the right
            // of the plot.
            var labelDiv = document.createElement('div');
            labelDiv.style.top = pos.pageY + 'px';
            labelDiv.style.left = (plotRef.getPlotOffset().left +
                plotRef.width() + 20) + 'px';
            labelDiv.classList.add('plot-label');
            labelDiv.textContent = traces[closestTraceIndex].label;
            document.body.appendChild(labelDiv);
            lastLabel = labelDiv;
            curHighlightedLine = traces[closestTraceIndex].label;
          }
          if (lastHighlightedLine != curHighlightedLine) {
            plotRef.draw();
          }
        }
      };
    }()));

    // escapeNewlines replaces newlines with <br />'s
    var escapeNewlines = function(str) {
      return (str + '').replace(/\n/g, '<br />');
    }

    // makeCommitBlock uses the timestamp and traceName, and fills in the
    // parentDiv with the relevant data. Returns true on success, false on no data.
    var makeCommitBlock = function(parentDiv, timestamp) {
      var noteText = '';
      var alwaysVisibleFields = [['author', 'Author'],
                    ['commit_msg', 'Commit message']];
      var hiddenFields = [['commit_msg', 'Commit message'],
                    ['commit_time', 'Commit time'],
                    ['hash', 'Commit hash'],
                    ['git_number', 'Git number']];
      var commit = commitData[timestamp];
      if (!commit) { return false; }
      var expandTemplate = $$$('#expandable').content.cloneNode(true);
      var visibleText = '';
      var hiddenText = '';
      alwaysVisibleFields.forEach(function(field) {
        if (commit[field[0]]) {
          if (field[0] != 'commit_msg') {
            visibleText += field[1] + ': ' + escapeNewlines(commit[field[0]]) +
                '<br />';
          } else {
            visibleText += 'Commit message: ' +
                commit['commit_msg'].split('\n')[0];
          }
        }
      });
      hiddenFields.forEach(function(field) {
        if (commit[field[0]]) {
          if (field[0] == 'commit_msg') {
            hiddenText += commit['commit_msg'].
                split('\n').slice(1).join('<br />') + '<br />';
          } else if (field[0] == 'hash') {
            var hashVal = commit['hash'];
            hiddenText += field[1] + ': ' + 
                '<a href=https://skia.googlesource.com/skia/+/' + hashVal + 
                '>' + hashVal + '</a><br />';
          } else {
            hiddenText += field[1] + ': ' + escapeNewlines(commit[field[0]]) +
                '<br />';
          }
        }
      });
      $$$('.visible', expandTemplate).innerHTML = visibleText;
      var expandableSpan = $$$('.expandable', expandTemplate);
      expandableSpan.innerHTML = hiddenText;
      parentDiv.appendChild(expandTemplate);
      return true;
    };

    $('#chart').bind('plotclick', function(evt, pos, item) {
      if(!item) { return; }
      var noteFragment = $$$('#plot-note').content.cloneNode(true);
      var note = $$$('.note', noteFragment);
      var topSection = $$$('.info', noteFragment);
      topSection.innerHTML = 'Trace: ' + item.series.label +
          '<br />Value: ' + item.datapoint[1] + '<br />';
      var timestamp = parseInt(item.datapoint[0] + '');
      if (!makeCommitBlock(topSection, timestamp)) {
        topSection.innerHTML += 'Commit time: ' + timestamp + '<br />';
      }
      // Get data from commits between this commit and the last one.
      var blamelistText = '';
      var seriesData = item.series.data;
      for (var i = -1; seriesData[i + 1][0] < parseInt(item.datapoint[0]) && i + 1 < seriesData.length; i++) {
      }
      if (i >= 0 && i < seriesData.length) {
        var moreInfo = $$$('.more-info', noteFragment);
        var lastTimestamp = item.series.data[i][0];
        var tailCommits = Object.keys(commitData).filter(function(timestamp) {
          return parseInt(timestamp) > lastTimestamp && 
              parseInt(timestamp) < item.datapoint[0];
        });
        var blamelistData = tailCommits.map(function(timestamp) {
          return commitData[timestamp + ''];
        });
        // Sort in descending order. I think.
        blamelistData.sort(function(a, b) {
          return b.commit_time - a.commit_time;
        });
        blamelistData.forEach(function(aCommit) {
          makeCommitBlock(moreInfo, aCommit.commit_time);
          moreInfo.appendChild(document.createElement('hr'));
        });
      }

      // Add annotations
      var timestampAsString = parseInt(item.datapoint[0]) + '';
      if(annotations[timestampAsString]) {
        var topNode = $$$('#messages', noteFragment);
        annotations[timestampAsString].forEach(function(annotation) {
          var annotationNode = $$$('#annotation').content.cloneNode(true);
          if(annotation['author']) {
            $$$('.author', annotationNode).textContent = annotation['author'];
          }
          if(annotation['notes']) {
            $$$('.notes', annotationNode).textContent = annotation['notes'];
          }
          // Set the text color based on the alert level.
          var wrapper = $$$('.wrapper', annotationNode);
          switch(annotation['type']) {
            case 2:
              wrapper.style.color = 'red';
              break;
            case 1:
              wrapper.style.color = 'dark yellow';
              break;
            default:
              wrapper.style.color = 'black';
          }
          topNode.appendChild(annotationNode);
        });
      }
      note.style.top = item.pageY + 10 + 'px';
      note.style.left = item.pageX + 10 + 'px';

      var removeChild = function(e) {
        var newActive = e.relatedTarget;
        console.log(newActive);
        while(newActive != null && newActive != note) {
          newActive = newActive.parentElement;
        }
        // Focus just moved from the element to inside the element;
        // stil good.
        if(newActive == note) { return; }

        document.body.removeChild(note);
        $$$('.make-solo', note).removeEventListener('click', hideOthers);
        $$$('.submit-annotation', note).removeEventListener('click', submitAnnotation);
        note.removeEventListener('blur', removeChild);
      };

      var hideOthers = function() {
        if (commitData[timestamp]) {
          for(var i = 0; i < traces.length; i++) {
            traces[i] = {
              data: traces[i].data,
              label: traces[i].label,
              lines: {
                show: traces[i].label == item.series.label
              }
            };
          }
        }
      };

      var submitAnnotation = function() {
        var submitRequest = new XMLHttpRequest();
        var annotationUsername = $$$('.username', note).value;
        var annotationMessage = $$$('.annotation-message', note).value;
        var annotationType = parseInt($$$('input[name=\"annotation-level\"]:checked',
            note).value);
        var annotationHash = '';
        if (commitData[timestampAsString]) {
          annotationHash = commitData[timestampAsString]['hash'];
        }
        if (annotationUsername.length === 0 || annotationMessage.length === 0 ||
            annotationHash.length === 0) {
          console.log('WARNING: At least one invalid field in annotation; not' +
              ' submitting');
          notifyUser('Please fill in all the annotation fields before' +
              ' submitting.');
          return;
        }
        var newAnnotation = {
          'id': -1,
          'type': annotationType,
          'author': annotationUsername,
          'notes': annotationMessage
        };
        console.log(JSON.stringify(newAnnotation));
        submitRequest.open('POST', 'annotations/');
        submitRequest.addEventListener('load', function() {
          if (submitRequest.status == 200) {
            if (!annotations[timestampAsString]) {
              annotations[timestampAsString] = [];
            }
            annotations[timestampAsString].push(newAnnotation);
            // TODO: Add to the note if it's still visible.
          } else if (submitRequest.status == 500) {
            console.log('ERROR: Annotation submit failed: ',
                submitRequest.responseText);
            notifyUser('Annotation submit failed: ' + 
                submitRequest.responseText);
          }
        });
        submitRequest.setRequestHeader('Content-Type', 
            'application/json;charset=UTF-8');
        submitRequest.send(JSON.stringify({
          'operation': 'add',
          'annotation': newAnnotation,
          'hashes': [annotationHash]
        }));
      };


      $$$('.submit-annotation', note).
          addEventListener('click', submitAnnotation);
      $$$('.make-solo', note).addEventListener('click', hideOthers);
      note.addEventListener('blur', removeChild, true);
      document.body.appendChild(note);
      note.focus();
    });

    $('#chart').bind('plotzoom', updateEdges);
    $('#chart').bind('plotpan', updateEdges);

    $$$('#zoom').setAttribute('min', -20);
    $$$('#zoom').setAttribute('max', 0);
    $$$('#zoom').setAttribute('step', 0.01);

    $$$('#reset-axes').addEventListener('click', function(e) {
      var options = plotRef.getOptions();
      var cleanAxes = function(axis) {
        axis.max = null;
        axis.min = null;
      };
      options.xaxes.forEach(cleanAxes);
      options.yaxes.forEach(cleanAxes);

      plotRef.setupGrid();
      plotRef.draw();

      e.preventDefault();
    });

    // Redraw the plot when traces are modified.
    Array.observe(traces, function(splices) {
      console.log(splices);
      plotRef.setData(traces);
      plotRef.setupGrid();
      plotRef.draw();

      var data = plotRef.getData();
      data.forEach(function(trace) {
        plotColors[trace.label] = trace.color;
      });
      updateEdges();
    });
    Object.observe(plotEdges, function() {
      if(plotEdges[0] != null) {
        $$$('#start').value = toRFC(plotEdges[0]);
      }
      if(plotEdges[1] != null) {
        $$$('#end').value = toRFC(plotEdges[1]);
      }
      if(plotEdges[0] != null && plotEdges[1] != null) {
        $$$('#zoom').value = -Math.log(plotEdges[1] - plotEdges[0]);
      }
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
      var req = new XMLHttpRequest();
      req.open('GET', 'annotations/?start=' + startTime + '&end=' + endTime);
      console.log('annotations/?start=' + startTime + '&end=' + endTime);
      req.addEventListener('load', function() {
        if(!req.response || req.status != 200) {
          return;
        }
        var data = req.response;
        if(req.responseType != 'json') {
          data = JSON.parse(req.response);
        }
        console.log(data);
        var commitToTimestamp = {};
        Object.keys(commitData).forEach(function(timestamp) {
          if (commitData[timestamp]['hash']) {
            commitToTimestamp[commitData[timestamp]['hash']] = timestamp;
          }
        });
        Object.keys(data).forEach(function(hash) {
          if (commitToTimestamp[hash]) {
            annotations[commitToTimestamp[hash]] = data[hash];
          } else {
            console.log('WARNING: Annotation taken for commit not stored in' +
                ' commitData');
          }
        });
        // Redraw to get the new lines
        plotRef.draw();
      });
      req.send();
    });

    $$$('#back-to-the-future').addEventListener('click', function(e) {
      var newMin = Date.parse($$$('#start').value)/1000;
      var newMax = Date.parse($$$('#end').value)/1000;
      if(isNaN(newMin) || isNaN(newMax)) {
        console.log('#back-to-the-future click handler: invalid input(s)');
      } else {
        plotEdges[0] = newMin;
        plotEdges[1] = newMax;
        var xaxis = plotRef.getOptions().xaxes[0];
        xaxis.min = plotEdges[0];
        xaxis.max = plotEdges[1];
        plotRef.setupGrid();
        plotRef.draw();
      }
    });
    $$$('#zoom').addEventListener('input', function() {
      var center = (plotEdges[0] + plotEdges[1])/2;
      var newRange = Math.exp(-$$$('#zoom').value);
      var xaxis = plotRef.getOptions().xaxes[0];
      plotEdges[0] = center - newRange/2;
      plotEdges[1] = center + newRange/2;
      xaxis.min = plotEdges[0];
      xaxis.max = plotEdges[1];
      plotRef.setupGrid();
      plotRef.draw();
    });

    $$$('#nuke-plot').addEventListener('click', function(e) {
      $$('option:checked').forEach(function(elem) {
        elem.selected = false;
      });
      traces.splice(0, traces.length);
      e.preventDefault();
    });
  }


  /**
   * Renders the legend and keeps it in sync with the visible traces.
   * {@code Legend} watches traces, and changes the elements inside of
   * #legend-table to match traces. Currently it removes all the elements
   * and regenerates them all from a template, but this seems to work well
   * enough for the time being.
   */
  function newLegend() {
    var legendTemplate = $$$('#legend-entry');
    Object.observe(traces, function(slices) {
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
        label.textContent = traceName;
        $$$('a', newLegendEntry).id = 'remove_' + traceName;
        $$$('.legend-box-inner', newLegendEntry).style.border = '5px solid ' +
            (plotColors[traceName] ? plotColors[traceName] : 'white');

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
        e.preventDefault();
      }
    });
    Object.observe(plotColors, function() {
      $$('#legend tr').forEach(function(traceElem) {
        // Get id, see if the color needs to be updated
        var traceName = $$$('input', traceElem).id;
        if(plotColors[traceName]) {
          var newStyleString = '5 px solid ' + plotColors[traceName];
          var innerBox = $$$('.legend-box-inner', traceElem);
          if(innerBox.style.border != newStyleString) {
            innerBox.style.border = newStyleString;
          }
        }
      });
    });
  }


  /**
   * Sets up the event handlers related to the query controls in the interface.
   * The callbacks in this function use and observe {@code queryInfo},
   * and modifies {@code traces}. Takes the object {@code Dataset} creates
   * as input.
   */
  function newQuery(dataset) {
    /**
     * Stores the store of DOM elements not currently visible.
     * {@code hiddenChildren} is used when a user enters text into one of the 
     * input boxes, to store the children don't meet their criteria.
     */
    var hiddenChildren = {};

    /**
     * Returns a list of strings of trace keys that match the selected options.
     * Checks the UI controls for the selected options, and uses queryInfo.allKeys
     * to find the ones that match it.
     */
    function getMatchingTraces() {
      var matching = [];
      var selectedOptions = new Array(queryInfo.params.length);

      function _push(elem) {
        selectedOptions[i].push(elem.value);
      }

      // Get relevant keys
      for(var i = 0; i < queryInfo.params.length; i++) {
        selectedOptions[i] = [];
        $$('#select_' + i + ' option:checked').forEach(_push);
      }
      console.log(selectedOptions);
      queryInfo.allKeys.forEach(function(key) {
        var splitKey = key.split(':');
        function _eqKey(e) {
          return e == splitKey[i];
        }
        var isMatching = true;
        for (var i = 0; i < selectedOptions.length; i++) {
          if (selectedOptions[i].length > 0) {
            if (!selectedOptions[i].some(_eqKey)) {
              isMatching = false;
              break;
            }
          }
        }
        if(isMatching) {
          matching.push(key);
        }
      });
      return matching;
    }


    /**
     * Returns a list of trybot result keys that match the selected options.
     * Behaves very similarly to getMatchingTraces().
     */
    function getMatchingTrybotTraces() {
      var matching = [];
      var selectedOptions = new Array(queryInfo.params.length);

      function _push(elem) {
        selectedOptions[i].push(elem.value);
      }

      // Get relevant keys
      for(var i = 0; i < queryInfo.params.length; i++) {
        selectedOptions[i] = [];
        $$('#select_' + i + ' option:checked').forEach(_push);
      }
      console.log(selectedOptions);
      Object.keys(queryInfo.trybotResults).forEach(function(key) {
        var splitKey = key.split(':');
        function _eqKey(e) {
          return e == splitKey[i];
        }
        var isMatching = true;
        for(var i = 0; i < selectedOptions.length; i++) {
          if(selectedOptions[i].length > 0) {
            if(!selectedOptions[i].some(_eqKey)) {
              isMatching = false;
              break;
            }
          }
        }
        if(isMatching) {
          matching.push(key);
        }
      });
      return matching;
    }


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

      function _len(p) { return p.length; }

      for (var i = 0; i < queryInfo.params.length; i++) {
        var column = document.createElement('td');

        var longest = Math.max.apply(null, queryInfo.params[i].map(_len));
        var minWidth = 0.75 * longest + 0.5;

        var input = document.createElement('input');
        input.id = 'input_' + i;
        input.style.width = minWidth + 'em';

        var header = document.createElement('h4');
        header.textContent = queryInfo.params[i][0];

        var select = document.createElement('select');
        select.id = 'select_' + i;
        select.style.width = minWidth + 'em';
        select.style.overflow = 'auto';
        select.setAttribute('multiple', 'yes');
        select.setAttribute('size', 16);

        for (var j = 1; j < queryInfo.params[i].length; j++) {
          var option = document.createElement('option');
          option.value = queryInfo.params[i][j];
          option.textContent = queryInfo.params[i][j].length > 0 ?
              queryInfo.params[i][j] : '(none)';
          select.appendChild(option);
        }

        column.appendChild(header);
        column.appendChild(input);
        column.appendChild(select);
        queryTable.appendChild(column);
      }
    }
    Object.observe(queryChange, onParamChange);

    /**
     * Updates the visible traces based on what the user inputs.
     * This appends traces to traces when the user presses the #add-lines
     * button. The addNewLines fields determine whether the query controls
     * will be checked and if the selected lines will be added.
     */
    var updateTraces = function(addNewLines) {
      // TODO: Chunk requests to improve efficiency, and not break
      // on loading too many traces at once.
      // TODO: Diff new trace list with old traces to ask for only what we need
      var allTraces = [];
      traces.forEach(function(t) { allTraces.push(t.label); });
      if(addNewLines) {
        getMatchingTraces().forEach(function(t) {
          if(allTraces.indexOf(t) == -1) {
            allTraces.push(t);
          }
        });
      }
      console.log(allTraces);

      var chunksLeft = 0;

      var newTileNums = [];
      var traceData = {};

      var processData = function(data) {
        console.log(data);
        // Process the new data.
        if(data['tiles']) {
          data['tiles'].forEach(function(tile) {
            console.log(tile);
            var num = tile['tileIndex'];
            if(newTileNums.indexOf(num) == -1) {
              newTileNums.push(num);
            }
            if(tile['traces']) {
              tile['traces'].forEach(function(trace) {
                if(!traceData[trace['key']]) {
                  traceData[trace['key']] = [];
                }
                traceData[trace['key']][num] = trace['data'];
              });
            }
          });
        }
      };

      var loadData = function(data) {
        processData(data);
        chunksLeft--;
        if (chunksLeft <= 0) {
          pushToTraces();
        }
      };

      var pushToTraces = function() {
        // Clear out the old trace data
        while(traces.length > 0) { traces.pop(); }

        console.log('newTileNums: ');
        console.log(newTileNums);
        newTileNums.sort(function(a, b) { return a - b; });
        // Put in the new tile numbers
        if(newTileNums.length >= dataset.tileNums.length) {
          console.log(newTileNums);
          while(dataset.tileNums.length > 0) { dataset.tileNums.pop(); }
          newTileNums.forEach(function(num) { dataset.tileNums.push(num); });
        }
        console.log(traceData);

        var newTraceNames = Object.keys(traceData);
        newTraceNames.forEach(function(traceName) {
          var mergedTraceData = [];
          // Combine the trace segments in the right order
          for(var i = 0; i < dataset.tileNums.length; i++) {
            if(traceData[traceName][dataset.tileNums[i]]) {
              Array.prototype.push.apply(mergedTraceData, 
                  traceData[traceName][dataset.tileNums[i]]);
            }
          }
          traces.push({
            data: mergedTraceData,
            label: traceName,
            lines: {
              show: true
            }
          });
        });
      };

      // Chunk the trace names into sets of 50, and send a new request for each
      // chunk. This is to avoid hitting the URI length limit.
      for (var i = 0; i < allTraces.length; i += 50) {
        chunksLeft++;
        dataset.requestTiles(loadData, {
          // This tells it to get trace data.
          'traces': allTraces.slice(i, i + 50),
          // These should get turned into strings
          // within requestTiles. 
          // Automatic type conversion!
          'omit_commits': true,
          'omit_params': true,
          'omit_names': true
        });
      }
    };

    // Add handlers to the query controls.
    $$$('#add-lines').addEventListener('click', function() {
      updateTraces(true);
    });
    $$$('#inputs').addEventListener('change', function(e) {
      var count = getMatchingTraces().length;
      var trybotCount = getMatchingTrybotTraces().length;
      $$$('#query-text').innerHTML = count + ' lines selected<br />' +
          trybotCount + ' lines with trybot results';
    });
    $$$('#inputs').addEventListener('input', function(e) {
      if(e.target.nodeName == 'INPUT') {
        var query = e.target.value.toLowerCase();
        var column = parseInt(e.target.id.slice('input_'.length));
        var possibleValues = queryInfo.params[column].slice(1).map(function(s) {
          return s.toLowerCase();
        });
        var results = possibleValues.filter(function(candidate) {
          return candidate.indexOf(query) != -1;
        });
        if(results.length < 1) {
          var matchLengths = possibleValues.map(function(candidate) {
            var maxMatch = 0;
            for(var start = 0; start < candidate.length; start++) {
              var i = 0;
              for(; start + i < candidate.length && query.length; i++) {
                if(candidate[start + i] != query[i]) {
                  break;
                }
              }
              if(i > maxMatch) { 
                maxMatch = i;
              }
            }
            return maxMatch;
          });
          var maxMatch = Math.max.apply(null, matchLengths);
          results = possibleValues.filter(function(_, idx) {
            return matchLengths[idx] >= maxMatch;
          });
        }
        if(!hiddenChildren[column]) { hiddenChildren[column] = []; }
        var removed = [];
        hiddenChildren[column].forEach(function(e, idx) {
          if(results.indexOf(e.value.toLowerCase()) != -1) {
            var selectChildren = $$$('#select_' + column).children;
            for(var i = 0; i < selectChildren.length; i++) {
              if(selectChildren[i].value > e.value) {
                $$$('#select_' + column).insertBefore(e, selectChildren[i]);
                removed.push(idx);
                return;
              }
            }
            $$$('#select_' + column).insertBefore(e, null);
            removed.push(idx);
          }
         });
        for(var i = removed.length - 1; i >= 0; i--) {
          hiddenChildren[column].splice(removed[i], 1);
        }
        $$('#select_' + column + ' option').forEach(function(e) {
          if(results.indexOf(e.value.toLowerCase()) == -1) {
            hiddenChildren[column].push(e);
            e.parentNode.removeChild(e);
          }
        });
      }
    });

    // resetSelect resets the given select (by name, such as '#issue') to
    // contain only one option with text '(none)' and value ''.
    function resetSelect(select) {
        var selectObj = $$$(select);
        while (selectObj.hasChildNodes()) {
          selectObj.removeChild(selectObj.lastChild);
        }
        var defaultOption = document.createElement('option');
        defaultOption.value = '';
        defaultOption.textContent = '(none)';
        defaultOption.selected = true;
        selectObj.appendChild(defaultOption);
    }

    // Update the list of trybot results, based on the tiles that we're currently over.
    Object.observe(commitData, function() {
      console.log('Updating trybot list');
      // Find the maximum and minimum timestamps to figure out the
      // {@code daysback} and {@code end} to send.
      var timestamps = Object.keys(commitData).map(function(ts) {return parseInt(ts);});
      var end = Math.max.apply(null, timestamps);
      var start = Math.min.apply(null, timestamps);
      var daysback = Math.ceil((end - start) / (60 * 60 * 24));
      console.log('trybots/nano?daysback=' + daysback + '&end=' + end);
      var req = new XMLHttpRequest();
      req.open('GET', 'trybots/nano?daysback=' + daysback + '&end=' + end);
      req.addEventListener('load', function() {
        if (!req.response || req.status != 200) {
          return;
        }
        var data = req.response;
        if (req.responseType != 'json') {
          data = JSON.parse(req.response);
        }
        console.log(data);
        // Clear out the old lists.
        var requesterOptions = $$$('#requester');
        resetSelect('#requester');
        var issueOptions = $$$('#issue');
        resetSelect('#issue');
        var trybotOptions = $$$('#trybot');
        resetSelect('#trybot');
        if (data['results'] && Array.isArray(data['results'])) {
          console.log('Refreshing trybot list');
          data['results'].sort();
          data['results'].forEach(function(requester) {
            if (requester['issues'] && requester['requester']) {
              var newROption = document.createElement('option');
              newROption.value = requester['requester'];
              newROption.textContent = requester['requester'];
              requesterOptions.appendChild(newROption);
              for (var issue in requester['issues']) {
                if (!requester['issues'].hasOwnProperty(issue) ||
                    !Array.isArray(requester['issues'][issue])) {
                  continue;
                }
                var newIOption = document.createElement('option');
                newIOption.value = issue;
                newIOption.textContent = issue;
                newIOption.dataset.requester = requester['requester'];
                issueOptions.appendChild(newIOption);
                requester['issues'][issue].sort();
                requester['issues'][issue].forEach(function(trybotResult) {
                  var newTOption = document.createElement('option');
                  newTOption.value = trybotResult;
                  newTOption.textContent = trybotResult;
                  newTOption.dataset.requester = requester['requester'];
                  newTOption.dataset.issue = issue;
                  trybotOptions.appendChild(newTOption);
                });
              }
            }
          });
        }
      });
      req.send();
    });

    // sortOptions sorts the options in the select name (such as '#issue')
    // given. Also resets the selected value to ''.
    function sortOptions(select) {
      var to_sort = [];
      $$(select + ' option').forEach(function(e) {
        to_sort.push(e);
      });
      to_sort.sort(function(a, b) {
        if (a.disabled === b.disabled) {
          if (a.value < b.value) {
            return -1;
          } else {
            return 1;
          }
        } else if (a.disabled) {
          return 1;
        } else {
          return -1;
        }
      });
      to_sort.forEach(function(i, idx) {
        $$$(select).appendChild(i);
        if (i.value === '') {
        }
      });
      $$$(select).value = '';
    }

    $$$('#requester').addEventListener('change', function() {
      var selectedRequester = $$$('#requester option:checked').value;
      $$('#issue option').forEach(function(e) {
        if (selectedRequester !== '' &&
            e.dataset.requester === selectedRequester) {
          e.disabled=false;
        } else {
          e.disabled=true;
        }
      });
      sortOptions('#issue');
    });

    $$$('#issue').addEventListener('change', function() {
      var selectedIssue = $$$('#issue option:checked').value;
      $$('#trybot option').forEach(function(e) {
        if (selectedIssue !== '' && e.dataset.issue === selectedIssue) {
          e.disabled=false;
        } else {
          e.disabled=true;
        }
      });
      sortOptions('#trybot');
    });

    $$$('#trybot').addEventListener('change', function() {
      queryInfo.trybotResults = {};

      var selectedResults = $$$('#trybot option:checked').value;
      if (selectedResults === '') {
        queryInfo.trybotResults = {};
        return;
      }
      // Grab the trybot results, and replace queryInfo.trybotResults with it.
      var req = new XMLHttpRequest();
      req.open('GET', 'trybots/nano/' + selectedResults);
      req.addEventListener('load', function() {
        if (!req.response || req.status != 200) {
          return;
        }
        var data = req.response;
        if (req.responseType != 'json') {
          data = JSON.parse(req.response);
        }
        if (data['traces'] && Array.isArray(data['traces'])) {
          data['traces'].forEach(function(trace) {
            if (trace['key'] && trace['data'] && trace['data'][0] &&
                trace['data'][0][1]) {
              queryInfo.trybotResults[trace['key']] = trace['data'][0][1];
            }
          });
        }
      });
      req.send();
    });


    // Tile control handlers
    $$$('#add-left').addEventListener('click', function(e) {
      if(dataset.tileNums[0] >= 1) {
        dataset.tileNums.unshift(dataset.tileNums[0] - 1);
        updateTraces(false);
      }
      e.preventDefault();
    });
    $$$('#shift-left').addEventListener('click', function(e) {
      var toRemove = [];
      for(var i = 0; i < dataset.tileNums.length; i++) {
        dataset.tileNums[i]--;
      }
      if(dataset.tileNums[0] < 0 && dataset.tileNums.length > 1) {
        dataset.tileNums.shift();
      }
      updateTraces(false);
      e.preventDefault();
    });
    $$$('#add-right').addEventListener('click', function(e) {
      dataset.tileNums.push(dataset.tileNums[dataset.tileNums.length - 1] + 1);
      updateTraces(false);
      e.preventDefault();
    });
    $$$('#shift-right').addEventListener('click', function(e) {
      for(var i = 0; i < dataset.tileNums.length; i++) {
        dataset.tileNums[i]++;
      }
      updateTraces(false);
      e.preventDefault();
    });
  }


  /**
   * Manages the set of keys the user can query over.
   * Returns an object containing a reference to requestTiles and
   * tileNums
   */
  function newDataset() {
    // TODO: Describe where these are used better
    // These describe the current "window" of data we're looking at.
    var tileNums = [-1];
    var scale = 0;

    /**
     * Helps make requests for a set of tiles.
     * Makes a XMLHttpRequest for using the data in 
     * {@code tileNums}, and {@code scale}, using the data in moreParams
     * as request query parameters. Calls finished with the data or
     * an empty object when finished.
     */
    var requestTiles = function(finished, moreParams) {
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
      if (moreParams) {
        Object.keys(moreParams).forEach(function(key) {
          params += encodeURIComponent(key) + '=' + encodeURI(moreParams[key]) + '&';
        });
      }

      request.open('GET', ['tiles', scale, tileNums.join(',')].
            join('/') + '?' + params);
      document.body.classList.add('waiting');
      request.addEventListener('load', onjsonload);
      request.addEventListener('error', onloaderror);
      request.addEventListener('loadend', onloadfinish);
      request.send();
    };


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
            }
            if (tile['names']) {
              tile['names'].forEach(function(name) {
                newNames[name] = true;
              });
              while (queryInfo.allKeys.length > 0) { queryInfo.allKeys.pop(); }
              var newNameList = Object.keys(newNames);
              for (var i = 0; i < newNameList.length; i++) {
                queryInfo.allKeys.push(newNameList[i]);
              }
            }
            if (tile['commits']) {
              tile['commits'].forEach(function(commit) {
                commitData[parseInt(commit['commit_time']) + ''] = commit;
              });
            }
          });
        }
       console.log('Dataset update end');
        queryChange.counter++;
      };

      requestTiles(processJSON, {});
    };

    Object.observe(tileNums, function() {
      console.log('tileNums change!');
      update();
    });


    update();
    return {
      'requestTiles': requestTiles,
      'tileNums': tileNums,
    };
  }


  /** microtasks
   *
   * Gets the Object.observe delivered.
   */
  function microtasks() {
    setTimeout(microtasks, 125);
  }


  function onLoad() {
    var dataset = newDataset();
    newQuery(dataset);
    newPlot();
    newLegend();

    microtasks();
    Object.observe(dataset.tileNums, function() {
      console.log('tileNums changed!');
    });
  }

  // If loaded via HTML Imports then DOMContentLoaded will be long done.
  if (document.readyState != 'loading') {
    onLoad();
  } else {
    window.addEventListener('load', onLoad);
  }

  return {
    traces: traces,
    plotColors: plotColors,
    queryInfo: queryInfo,
    commitData: commitData,
    $$: $$,
    $$$: $$$
  };

}());
