/**
 * Copyright (c) 2014 The Chromium Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file */
/** Provides the logic behind the performance visualization webpage. */

var KEY_DELIMITER = ':';
var NUM_SHOW_RESULTS = 100000;
var MIN_XRANGE = 3600;
var plotRef = null;
var numBitmapsLeft = 0;

function last(ary) {
  return ary[ary.length - 1];
}


var commitToTimestamp = {};
var commits = [];
var currentTimestamp = null;

function timestampToCommit(timestamp) {
  for (var i = 0; i < commits.length; i++) {
    if (commitToTimestamp[commits[i]] == timestamp) {
      return commits[i];
    }
  }
  return '';
}


function getLegendKeys() {
  var keys = [];
  $('#legend input').each(function() {
    keys.push(this.id);
  });
  console.log('getLegendNames(): ' + keys);
  return keys;
}


function addToLegend(key) {
  if ($('#legend').find('[id=\'' + key + '\']').length > 0) { return; }
  return $('#legend table tbody').append('<tr><td><input type=checkbox id=' +
      key + ' checked>' + key + '</input></td>' +
      '<td><a href=#target id=' + key +
      '_remove>Remove</a></td></tr>');
}


function removeFromLegend(key) {
  console.log('Removing from legend: ' + key);
  plotData.makeInvisible(key);
  $('#legend table tbody').find('[id=\'' + key + '\']').
      parent().parent().remove();
  addHistory();
}


var plotData = (function() {
  var data = {};
  var visibleKeys = [];     // List of currently visible lines
  var cachedKeys = [];     // List of currently cached lines

  function getKeys() {
    return cachedKeys;
    // TODO: Replace with something better when the server is smarter.
  }


  function isCached(key) {
    return cachedKeys.indexOf(key) != -1;
  }


  function isVisible(key) {
    return visibleKeys.indexOf(key) != -1;
  }


  function addLineData(key, newData) {
    if (!isCached(key)) {
      data[key] = newData;
      cachedKeys.push(key);
    } else {
      // NOTE: This may be a performance bottleneck. Need more data.
      // This may also cause issues
      // if the server fails to respect data ranges
      data[key].push.apply(newData);
      data[key].sort();
    }
  }


  function getLine(key, callback) {
    if (isCached(key)) {
      // TODO: Get more data if the current range is partially empty
      callback(data[key]);
    }
    // TODO: Query the server for more trace data
    console.log('WARNING: Querying for individual traces not' +
        'currently implemented');
  }

  return {
    /** Returns the list of visible lines. This is a reference, so modifications
     * to it change the visibility of the plot lines. */
    getVisibleKeys: function() {return visibleKeys;},


    /** Returns the data in a FLOT-readable manner. */
    getProcessedPlotData: function() {
      var outOfBoundPoints = [];
      var lines = visibleKeys.map(function(key) {
        return {label: key,
            data: data[key],
            color: cachedKeys.indexOf(key)};
      });
      var maxLines = Math.max.apply(null, lines.map(function(series) {
        return Math.max.apply(null, series.data.map(function(e) {
          return e[0];
        }));
      }));
      return lines;
    },


    /** Adds a line to the graph. */
    makeVisible: function(key, nodraw) {
      if (isCached(key)) {
        if (!isVisible(key)) {
          visibleKeys.push(key);
          if (!nodraw) {
            plotStuff(this);
          }
        }
      } else {
        console.log('makeVisible: uncached line requested.');
        this.getAndAddLine(key);
      }
    },


    /** Removes a line from the graph. */
    makeInvisible: function(key) {
      if (isVisible(key)) {
        visibleKeys.splice(visibleKeys.indexOf(key), 1);
        plotStuff(this);
      }
    },


    /** Adds a line to the graph, calling the callback after it's successfully
     * been loaded. */
    getAndAddLine: function(key, callback, nodraw) {
      console.log(key);
      var _this = this;
      getLine(key, function(newData) {
        addLineData(key, newData);
        addToLegend(key);
        _this.makeVisible(key, nodraw);
        if (!nodraw) {
          plotStuff(this);
        }
        if (callback) {
          callback();
        }
      });
    },


    
    /** Gets the data of the given dataset type, and loads the data into its
     * cache, as well as passing appropriate data to the schema object
     * for its use.
     */
    loadData: function(schema_type) {
       $.getJSON('json/' + schema_type, function(json) {
        var orderedTimestamps = [];
        if (json['param_set']) {
          schema.load(schema_type, json['param_set']);
        }
        json['commits'].forEach(function(commit) {
          var hash = commit['hash'];
          var timeStamp = commit['commit_time'];
          if (hash && timeStamp && (commits.indexOf(hash) == -1)) {
            commits.push(hash);
            commitToTimestamp[hash] = timeStamp;
          }
          orderedTimestamps.push(timeStamp);
        });
        var orderedKeys = schema.getKeys();
        json['traces'].forEach(function(trace) {
          var line = [];
          var values = trace['values'];
          if (!trace['key']) return;
          for (var i = 0; i < values.length; i++) {
            if (values[i] > 1e+40) {
              continue;
            }
            line.push([orderedTimestamps[i], values[i]]);
          }
          var keys = [];
          for (var key in trace['params']) {
            if (trace['params'].hasOwnProperty(key)) {
              if (orderedKeys.indexOf(key) != -1) {
                keys[orderedKeys.indexOf(key)] =
                    trace['params'][key];
              }
            }
          }
          // Set all the unused values to empty strings.
          for (var i = 0; i < orderedKeys.length; i++) {
            if (!keys[i]) {
              keys[i] = '';
            }
          }
          schema.check(keys);

          var newKey = keys.join(KEY_DELIMITER);
          addLineData(newKey, line);
        });
        schema.updateSchema();
      });
    },

    
    /** Returns the list of lines currently selected in the option boxes.*/
    getAvailableLines: function() {
      var selected = $('#line-form').children().children('select').
          map(function(idx, e) {
        return {
          id: idx,
          validElems: $(this).children(':selected').map(function() {
            return this.value;
          }).get() };
      }).get().filter(function(elem) {
        return elem.validElems.length > 0;
      });
      var validKeys = getKeys().map(function(key) {
        return key.split(KEY_DELIMITER);
      });
      selected.forEach(function(keyset) {
        validKeys = validKeys.filter(function(key) {
          return keyset.validElems.indexOf(key[keyset.id]) != -1;
        });
      });
      return validKeys;
    },


    /** Returns the private variables of the function. Useful for console
     * debugging. */
    debug: function() {
      return {
        data: data,
        visibleKeys: visibleKeys,
        cachedKeys: cachedKeys,
      };
    }
  };
})();


/** Updates the plot with new data. */
function plotStuff(source) {
  if (plotRef) {
    var lines = source.getProcessedPlotData();
    updateSlidersFromChart(lines);
    plotRef.setData(lines);
    var options = plotRef.getOptions();
    options.xaxes.forEach(function(axis) {
      axis.max = null;
      axis.min = null;
    });
    options.yaxes.forEach(function(axis) {
      axis.max = null;
      axis.min = null;
    });
    plotRef.setupGrid();
    plotRef.draw();
  }
}

var zoomMin = null;
var zoomMax = null;


/** Using the data from the plot reference it's passed, it sets the horizontal
 * zoom controls to appropriate settings. */
function updateSlidersFromChart(plotData) {
  var newSliderMax;
  var newSliderMin;
  var newSliderZoomMaxSet;
  var newSliderZoomMinSet;
  console.log('slider update from chart');
  // Assume default settings
  if (plotData) {
    var data = plotData.map(function(series) {
      return series.data;
    });
    newSliderMin = Math.min.apply(null, data.map(function(set) {
      return Math.min.apply(null, set.map(function(point) {
        return point[0];
      }));
    }));
    newSliderMax = Math.max.apply(null, data.map(function(set) {
      return Math.max.apply(null, set.map(function(point) {
        return point[0];
      }));
    }));
    $('#min-zoom, #max-zoom').attr('min', newSliderMin);
    $('#min-zoom, #max-zoom').attr('max', newSliderMax);
  }

  var xaxis = plotRef.getOptions().xaxes[0];
  if (xaxis.min == null || xaxis.max == null) {
    console.log('axes reset');
    newSliderZoomMinSet = newSliderMin;
    newSliderZoomMaxSet = newSliderMax;
  } else {
    newSliderZoomMinSet = xaxis.min;
    newSliderZoomMaxSet = xaxis.max;
  }
  $('#min-zoom').val(newSliderZoomMinSet);
  $('#min-zoom-value').val(newSliderZoomMinSet);
  $('#max-zoom').val(newSliderZoomMaxSet);
  $('#max-zoom-value').val(newSliderZoomMaxSet);
}


/** Sets the plot's horizontal zoom to match that given in the sliders.*/
function updateChartFromSliders() {
  var xMin = $('#min-zoom').val();
  var xMax = $('#max-zoom').val();
  var xaxis = plotRef.getOptions().xaxes[0];
  xaxis.min = xMin;
  xaxis.max = xMax;
  plotRef.setupGrid();
  plotRef.draw();
}


/** Sets the sliders to match the zoom in the plot.*/
function updateSlidersFromZoom() {
  var xaxis = plotRef.getOptions().xaxes[0];
  var xMin = xaxis.min;
  var xMax = xaxis.max;
  $('#min-zoom').val(xMin);
  $('#min-zoom-value').val(xMin);
  $('#max-zoom').val(xMax);
  $('#max-zoom-value').val(xMax);
}

var lastHighlightedPoint = null;


/** Initializes the FLOT plot. */
function plotInit() {
  if (!plotRef) {
    plotRef = $('#chart').plot(plotData.getProcessedPlotData(),
        {
          legend: {
            labelFormatter: function(label) {
              if (label.slice(0, 2) != '__') {
                return label;
              } else {
                return null;
              }
            },
            sorted: true
          },
          grid: {
            hoverable: true,
            autoHighlight: true,
            mouseActiveRadius: 10,
            clickable: true
          },
          xaxis: {
            tickFormatter: function(val, axis) {
              var valDate = new Date(val * 1000);
              return valDate.toString();
            }
          },
          yaxis: {
            /* zoomRange: false */
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
    $('#chart').bind('plotclick', function(evt, pos, item) {
      $('#note').hide();
      if (lastHighlightedPoint) {
        plotRef.unhighlight(lastHighlightedPoint.series,
          lastHighlightedPoint.datapoint);
      } else {
        lastHighlightedPointed = null;
      }
      if (item) {
        currentTimestamp = item.datapoint[0];

        plotRef.highlight(item.series, item.datapoint);
        lastHighlightedPoint = item;
        makePopup(evt, pos, item);
      }
    });

    $('#chart').bind('plotpan', updateSlidersFromZoom);
    $('#chart').bind('plotzoom', updateSlidersFromZoom);
  }
}


/** Makes the plot popup. */
function makePopup(evt, pos, item) {
  // Move the div with id "notes" to the desired point,
  // and post useful information in there
  var notePad = $('#note');
  notePad.hide();
  notePad.css({'top': item.pageY + 10, 'left': item.pageX});
  // TODO: add more useful data
  $('#note #data').html('Timestamp: ' + item.datapoint[0] + '<br />value: ' +
      item.datapoint[1] + '');
  notePad.show();
}


/** Sends the user a notification on the bottom bar. */
function notifyUser(text, replace) {
  if (!$('#notification').is(':visible') && !replace) {
    $('#notification-text').html(text);
    $('#notification').show().delay(5000).fadeOut(1000).hide(10);
  } else {
    window.setTimeout(notifyUser, 400, text);
  }
}


/** Manages the schema for the option bar and keys for the plot data. */
var schema = (function() {
  var schemaType = '';
  var curSchema = {};
  var otherSchemas = [];
  var otherSchemaData = {};
  return {

    /** Loads data into the schema. */
    load: function(newName, data) {
      var omitFields = ['arch', 'role', 'os', 'gpu', 'configuration',
          'model', 'badParams', 'bbh', 'mode', 'skpSize',
          'viewport', 'extraConfig'];
      var expectationTypes = ['upper', 'lower', 'expected',
          'upper_wall', 'lower_wall', 'expected_wall'];
      var newSchema = {};

      schemaType = newName;
      for (var key in data) {
        if (data.hasOwnProperty(key) &&
            omitFields.indexOf(key) == -1) {
          newSchema[key] = data[key];
        }
      }
      // TODO: Store the old schema data somewhere,
      // if there is already a schema
      curSchema = newSchema;
      if (curSchema['measurementType']) {
        var newType = curSchema['measurementType'].filter(
          function(e) {
            console.log(e);
            return expectationTypes.indexOf(e) == -1;
          }
        );
        curSchema['measurementType'] = newType;
      }
      console.log(curSchema);
    },

    /** Returns a list of keys in the schema. */
    getKeys: function() {
      var keys = [];
      for (var key in curSchema) {
        if (curSchema.hasOwnProperty(key)) {
          keys.push(key);
        }
      }
      return keys;
    },

    /** Updates the option boxes to reflect the current state of the schema. */
    updateSchema: function() {
      var keys = this.getKeys();
      var _this = this;
      console.log(keys);
      $('#line-form').children().remove();
      keys.forEach(function(key) {
        curSchema[key].sort();
        var width = Math.max.apply(null, curSchema[key].
            map(function(k) {
          console.log(k);
          return k.length;
        }));
        $('#line-form').append(
          '<td>' +
            '<input id=\"' + key + '-name\"' +
              ' style=\" width:' + (0.5 * width + 1) + 'em;\"' +
              ' autocomplete=on>' +
            '</input>' +
            '<select id=\"' + key + '-results\" style=\"' +
              'width:100%;overflow:auto;\" multiple=\"yes\">' +
            '</select>' +
          '</td>');
        var inputHandler = (function(safeKey) {
          return function() {_this.updateOptions(safeKey);};})(key);
        // NOTE: This may break.
        document.getElementById(key + '-name').addEventListener('input',
            inputHandler);
      }, this);
      keys.forEach(function(key) {_this.updateOptions(key)});
    },

    /** Updates the individual selection in the options box of the given id
     * to reflect the string the user has entered. */
    updateOptions: function(id) {
      var query = $('#' + id + '-name').val();
      var results = curSchema[id].filter(function(candidate) {
        return candidate.slice(0, query.length) == query;
      });  // TODO: If this is too slow, swap with binary search
      if (results.length < 1) {
        matchLengths = curSchema[id].map(function(candidate) {
          var i = 0;
          var minLen = Math.min(candidate.length, query.length);
          for (i = 0; i < minLen; i++) {
            if (candidate[i] != query[i]) {
              break;
            }
          }
          return i;
        });
        maxMatch = Math.max.apply(null, matchLengths);
        results = curSchema[id].filter(function(_, idx) {
          return matchLengths[idx] >= maxMatch;
        });
      }
      if (curSchema[id].filter(function(val) {
          return val == '';}).length > 0) {
        results.push('');
      }
      $('#' + id + '-results').html(results.map(function(c) {
        return '<option value=' + c + '>' + c + '</option>';
      }).join(''));
      var updateStuff = function() {
        // NOTE: Very inefficient right now.
        var lines = plotData.getAvailableLines();
        $('#line-num').html(lines.length + ' lines selected.');
        if (lines.length == 0) return;
        var options = new Array(lines[0].length);
        var keys = schema.getKeys();
        for (var i = 0; i < options.length; i++) {
          options[i] = curSchema[keys[i]].slice(0);
        }
        lines.forEach(function(l) {
          for (var i = 0; i < options.length; i++) {
            if (options[i].indexOf(l[i]) != -1) {
              options[i].splice(options[i].indexOf(l[i]), 1);
            }
          }
        });
        for (var i = 0; i < options.length; i++) {
          var elements = $('#' + keys[i] + '-results').children();
          elements.each(function() {
            if (!this.getAttribute('disabled') &&
                options[i].indexOf(this.value) != -1) {
              if (keys[i] == id) return;
              this.setAttribute('disabled', true);
            } else if (this.getAttribute('disabled') &&
                options[i].indexOf(this.value) == -1) {
              this.disabled = false;
            }
          });
        }
      };
      updateStuff();
      document.getElementById(id + '-results').addEventListener('change',
              updateStuff);
    },

    /** Adds in any keys the schema may be missing that appear in the
     * given key. */
    check: function(keys) {
      var order = this.getKeys();
      // console.log(keys);
      for (var i = 0; i < order.length; i++) {
        if (curSchema[order[i]].indexOf(keys[i]) == -1 &&
            order[i] != 'measurementType') {
          console.log(keys[i]);
          curSchema[order[i]].push(keys[i]);
        }
      }
    },

    /** Returns the private variables for debugging purposes. */
    debug: function() {
      return {
        schemaType: schemaType,
        curSchema: curSchema,
        otherSchemas: otherSchemas,
        otherSchemaData: otherSchemaData
      };
    }
  };
})();


/** Updates the page history to match the currently selected lines. */
function updateHistory() {
  var legend = getLegendKeys();
  var historyState = {
    legendState: legend,
    visibleState: plotData.getVisibleKeys()
  };
  window.history.replaceState(historyState, 'foo', window.location.href);
  // TODO: Encode the state in the url as well.
}


/** Adds a page history state to match the change in selected lines. */
function addHistory() {
  var legend = getLegendKeys();
  var historyState = {
    legendState: legend,
    visibleState: plotData.getVisibleKeys()
  };
  console.log(historyState);
  window.history.pushState(historyState, 'foo', window.location.href);
}


// Make the plot
window.addEventListener('load', function() {
  // Load JSON with hash to timestamp conversions.
  // May or may not be needed in the end, but
  plotInit();
  plotData.loadData('perf');

  $('#notification').hide();
  $('#note').hide();

  // Add trigger for checkmarks
  document.getElementById('legend').
        addEventListener('click', function(e) {
    console.log('legend click');
    var target = e.target;
    if ((target.nodeName == 'INPUT') &&
        (target.type == 'checkbox')) {
      if (!target.checked) {
        plotData.makeInvisible(target.id);
      } else {
        plotData.makeVisible(target.id);
       }
    } else if (target.nodeName == 'A') {
      removeFromLegend(target.id.slice(0, -'_remove'.length));
      e.preventDefault();
    }
  });

  document.body.addEventListener('click', function(e) {
    console.log('body click');
    if (!$(e.target).parents().is('#note,#chart')) {
      $('#note').hide();
    }
  });

  // TODO: Make work with schema, and also add in history support.
  // Add trigger for adding lines to the graph.
  document.getElementById('add-lines').addEventListener('click', function() {
    console.log('add click');
    plotData.getAvailableLines().forEach(function(ary) {
      plotData.getAndAddLine(ary.join(KEY_DELIMITER), undefined, true);
    });
    plotStuff(plotData);
    addHistory();
    return false;
  });

  var zoomChangeHandler = function() {
    console.log('zoom change');
    if (plotData.getVisibleKeys().length > 0) {
      var newMin = $('#min-zoom').val();
      var newMax = $('#max-zoom').val();
      if (newMin > newMax) {
        if (newMax = this.value) {
          newMin = Math.max(newMax - MIN_XRANGE, $('#min-zoom').
            attr('min'));
        } else {
          newMax = Math.min(newMin + MIN_XRANGE, $('#max-zoom').
            attr('max'));
        }
      }
      $('#min-zoom-value').val(newMin);
      $('#max-zoom-value').val(newMax);
      $('#min-zoom').val(newMin);
      $('#max-zoom').val(newMax);
      updateChartFromSliders();
    }
  };
  document.getElementById('min-zoom').
        addEventListener('input', zoomChangeHandler);
  document.getElementById('max-zoom').
        addEventListener('input', zoomChangeHandler);

  var zoomBlurHandler = function() {
    console.log('zoom change');
    if (plotData.getVisibleKeys().length > 0) {
      var newMin = parseInt($('#min-zoom-value').val());
      var newMax = parseInt($('#max-zoom-value').val());
      if (isNaN(newMin) || isNaN(newMax) ||
          newMin < $('#min-zoom').attr('min') ||
          newMax > $('#max-zoom').attr('max')) {
        console.log('invalid input');
        notifyUser('Invalid input');
        return;
      }
      var realMin = Math.min(newMin, newMax);
      var realMax = Math.max(newMin, newMax);
      $('#min-zoom').val(realMin);
      $('#max-zoom').val(realMax);
      updateChartFromSliders();
    }
  };
  document.getElementById('min-zoom-value').
      addEventListener('blur', zoomBlurHandler);
  document.getElementById('max-zoom-value').
      addEventListener('blur', zoomBlurHandler);


  Array.prototype.forEach.call(document.getElementsByName('schema-type'), 
          function(e) {
    e.addEventListener('change', function() {
      console.log('schema change');
      var newSchema = this.value;
      plotData.loadData(newSchema);
    });
  });

  window.addEventListener('popstate', function(event) {
    console.log(event);
    var state = event.state;
    if (state && state.legendState && state.visibleState) {
      // TODO: Do this more intelligently.
      var newLegend = state.legendState;
      var newVisible = state.visibleState;
      while (plotData.getVisibleKeys().length > 0) {
        plotData.makeInvisible(last(plotData.getVisibleKeys()));
      }
      $('#legend table tbody').children().remove();
      for (var i = 0; i < newLegend.length; i++) {
        console.log('Adding ' + newLegend[i] + 'to legend');
        addToLegend(newLegend[i]);
      }
      for (var j = 0; j < newVisible.length; j++) {
        plotData.makeVisible(newVisible[j]);
      }
    }
    plotStuff(plotData);
  });
});

