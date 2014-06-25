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


/** A safe wrapper around a dictionaryish object */
function PagedDictionary(attrs) {
  var dict = {};
  var index = [];
  var current = null;
  // Adds extra attributes to extend functionality as needed
  for(var attr in attrs) {
    if(attrs.hasOwnProperty(attr)) {
      this[attr] = attrs[attr];
    }
  }
  /* Returns true if the dictionary has something with that index. */
  this.has = function(id) {
    return index.indexOf(id) != -1;
  };
  /* Returns the value currently being pointed to. */
  this.cur = function() {
    if(current) {
      return dict[current];
    } else {
      return null;
    }
  };
  /* Returns the current key being used as a pointer. */
  this.currentId = function() {
    return current;
  };
  /* Returns the value matching the key if it exists, null otherwise. */
  this.get = function(id) {
    if(this.has(id)) {
      return dict[id];
    } else {
      return null;
    }
  };
  /* Adds a value to the dictionary. */
  this.add = function(id, val) {
    if(!this.has(id)) {
      index.push(id);
    }
    dict[id] = val;
  };
  /* Adds a value to the dictionary, and has the current pointer point to it. */
  this.push = function(id, val) {
    this.add(id, val);
    this.makeCurrent(id);
  };
  /* Points current at a particular value. */
  this.makeCurrent = function(id) {
    if(this.has(id)) {
      current = id;
      return true;
    } else {
      return false;
    }
  };
  /* Returns a list of existing keys. */
  this.index = function() {
    return index;
  };
  // TODO: Make map function
  /* Removes a value from the dictionary. */
  this.remove = function(id) {
    if(!this.has(id)) {
      dict[id] = null;
      index.splice(index.indexOf(id),1);
    }
  };
  /* Sets a value for a given key, returns false if it fails. */
  this.set = function(id, val) {
    if(this.has(id)) {
      dict[id] = val;
      return true;
    } else {
      return false; 
    }
  };
  /* Returns the position of a key in the key list. */
  this.indexOf = function(idx) {
    return index.indexOf(idx);
  }
  /* Looks up the index for a given item. */
  this.rlookup = function(val) {
    for(var i = 0; i < dict.length; i++) {
      if(this.dict[index[i]] == val) {
        return index[i];
      } else {
        return null;
      }
    }
  };
  /* Returns the private variables for debugging purposes. */
  this.debug = function() {
    return {dict: dict, index: index, current: current};
  };
}


var commitData = {};
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

  return $('#legend table tbody').append(
      '<tr><td><input type=checkbox id=' + key + ' checked></input>' +
        '<div class=legend-box-outer>' +
            '<div class=legend-box-inner>' + 
            '</div>'+
        '</div>'+
          key + '</td>' +
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
  var dict = new PagedDictionary();
  // Dictionary of datasets. Each dataset should be an object like 
  // {
  //    data: new PagedDictionary(), // Dictionary of cached datasets
  //    visibleKeys: []             // Visible traces
  // }
  // TODO(kelvinly): Replace dictionary with WeakMap

  function cur() {
    return dict.cur();
  }


  function getKeys() {
    return cur().index();
  }


  function isCached(key) {
    return cur().has(key);
  }


  function isVisible(key) {
    return cur().visibleKeys.indexOf(key) != -1;
  }


  function addLineData(key, newData) {
    if (!isCached(key)) {
      cur().add(key, newData);
    } else {
      // NOTE: This may be a performance bottleneck. Need more data.
      // This may also cause issues
      // if the server fails to respect data ranges
      cur().get(key).push.apply(newData);
      cur().get(key).sort();
    }
  }


  function getLine(key, callback) {
    if (isCached(key)) {
      // TODO: Get more data if the current range is partially empty
      callback(cur().get(key));
    }
    // TODO: Query the server for more trace data
    console.log('WARNING: Querying for individual traces not' +
        'currently implemented');
  }

  return {
    /** Returns the list of visible lines. This is a reference, so modifications
     * to it change the visibility of the plot lines. */
    getVisibleKeys: function() { return cur().visibleKeys; },


    /** Returns the data in a FLOT-readable manner. */
    getProcessedPlotData: function() {
      var outOfBoundPoints = [];
      var lines = cur().visibleKeys.map(function(key) {
        return {
          label: key,
          data: cur().get(key),
          color: cur().indexOf(key)};
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
          cur().visibleKeys.push(key);
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
    makeInvisible: function(key, nodraw) {
      if (isVisible(key)) {
        cur().visibleKeys.splice(cur().visibleKeys.indexOf(key), 1);
        if(!nodraw) {
          plotStuff(this);
        }
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
    loadData: function(schema_type, callback) {
      console.log('Loading data for ' + schema_type);
      // Check to see if it's already been loaded or not
      if(dict.has(schema_type)) {
        dict.makeCurrent(schema_type);
        schema.switchSchema(schema_type);

        $('#legend table tbody').children().remove();
        cur().visibleKeys.forEach(function(key) {
          addToLegend(key);
        });
        plotStuff(this);

        if(callback) {
          callback();
        }
      }
      $.getJSON('json/' + schema_type, function(json) {
        dict.push(schema_type, new PagedDictionary({
          visibleKeys: []
        }));
        console.log(json);
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
            if(commit['commit_msg']) {
              commitData[hash] = commit['commit_msg'];
            }
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
          // console.log('adding line ' + newKey);
          addLineData(newKey, line);
        });
        schema.updateSchema();

        if(callback) {
          callback();
        }
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
      return dict;
    }
  };
})();


/** Updates the plot with new data. */
function plotStuff(source) {
  if (plotRef) {
    var lines = source.getProcessedPlotData();
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
    if(source.getVisibleKeys().length > 0) {
      updateSlidersFromChart(lines);
    }
    // Update the legend's colors
    $('#legend input').each(function() {
      var color = 'white';
      var _this = this;
      plotRef.getData().forEach(function(series) {
        if(series.label == _this.id) {
          color = series.color;
        }
      });
      this.parentElement.getElementsByClassName('legend-box-inner')[0].
            style.border = '5px solid ' + color;
    });
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
  $('#min-zoom-value').val(toRFC(newSliderZoomMinSet));
  $('#max-zoom').val(newSliderZoomMaxSet);
  $('#max-zoom-value').val(toRFC(newSliderZoomMaxSet));
}


/** Returns the datetime compatible version of a POSIX timestamp.*/
function toRFC(timestamp) {
  // Slice off the ending 'Z'
  return new Date(timestamp*1000).toISOString().slice(0, -1);
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
  $('#min-zoom-value').val(toRFC(xMin));
  $('#max-zoom').val(xMax);
  $('#max-zoom-value').val(toRFC(xMax));
}

var lastHighlightedPoint = null;


/** Initializes the FLOT plot. */
function plotInit() {
  if (!plotRef) {
    plotRef = $('#chart').plot(plotData.getProcessedPlotData(),
      {
          legend: {
            show: false
          },
          grid: {
            hoverable: true,
            autoHighlight: true,
            mouseActiveRadius: 10,
            clickable: true
          },
          xaxis: {
            ticks: function(axis) {
              var range = axis.max - axis.min;
              // Different possible tick intervals, ranging from a second to
              // about a year
              var scaleFactors = [1, 2, 3, 5, 10, 15, 20, 30, 45, 60, 2*60, 
                                  4*60, 5*60, 15*60, 20*60, 30*60, 
                                  60*60, 2*60*60, 3*60*60, 4*60*60,
                                  5*60*60, 6*60*60, 12*60*60, 24*60*60, 
                                  7*24*60*60, 30*24*60*60, 2*30*24*60*60,
                                  4*30*24*60*60, 6*30*24*60*60, 365*24*60*60];
              var MAX_TICKS = 5;
              var i = 0;
              while(range/scaleFactors[i] > MAX_TICKS && i < scaleFactors.length) {
                i++;
              }
              var scaleFactor = scaleFactors[i];
              var cur = scaleFactor*Math.ceil(axis.min/scaleFactor);
              var ticks = [];
              do {
                var tickDate = new Date(cur*1000);
                var formattedTime = tickDate.toString();
                if(scaleFactor >= 24*60*60) {
                  formattedTime = tickDate.toDateString();
                } else {
                  // TODO: Find a way to make a string with only the hour or minute
                  formattedTime = tickDate.toDateString() + '<br \\>' +
                    tickDate.toTimeString();
                }
                ticks.push([cur, formattedTime]);
                cur += scaleFactor;
              } while(cur < axis.max);
              return ticks;
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
  var hash = timestampToCommit(item.datapoint[0]);
  var hashData = '';
  var commitMsg = '';
  if(hash.length > 0 && commitData[hash]) {
    hashData = 'hash: ' + hash + '<br />';
    commitMsg = 'commit message: ' + commitData[hash];
  }
  $('#note #data').html(
      hashData +
      'timestamp: ' + item.datapoint[0] + '<br />' +
      'value: ' + item.datapoint[1] + '<br />' +
      commitMsg
      );
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
  var schemaData = new PagedDictionary();
  return {

    /** Loads data into the schema. */
    load: function(newName, data) {
      var omitFields = ['arch', 'role', 'os', 'gpu', 'configuration',
          'model', 'badParams', 'bbh', 'mode', 'skpSize',
          'viewport', 'extraConfig'];
      var expectationTypes = ['upper', 'lower', 'expected',
          'upper_wall', 'lower_wall', 'expected_wall'];
      var newSchema = {};

      for (var key in data) {
        if (data.hasOwnProperty(key) &&
            omitFields.indexOf(key) == -1) {
          newSchema[key] = data[key];
        }
      }
      if (newSchema['measurementType']) {
        var newType = newSchema['measurementType'].filter(
          function(e) {
            console.log(e);
            return expectationTypes.indexOf(e) == -1;
          }
        );
        newSchema['measurementType'] = newType;
      }
      schemaData.push(newName, newSchema);
      console.log(schemaData.cur());
    },

    /** Switches to a different schema*/
    switchSchema: function(newSchemaName) {
      schemaData.makeCurrent(newSchemaName);
      this.updateSchema();
    },

    /** Returns a list of keys in the schema. */
    getKeys: function() {
      var keys = [];
      for (var key in schemaData.cur()) {
        if (schemaData.cur().hasOwnProperty(key)) {
          keys.push(key);
        }
      }
      return keys;
    },


    /** Returns the name of the currently loaded schema. */
    getCurrentSchema: function() {
      return schemaData.currentId();
    },


    /** Updates the option boxes to reflect the current state of the schema. */
    updateSchema: function() {
      var keys = this.getKeys();
      var _this = this;
      $('#line-form').children().remove();
      keys.forEach(function(key) {
        schemaData.cur()[key].sort();
        var width = Math.max.apply(null, schemaData.cur()[key].
            map(function(k) {
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
      var results = schemaData.cur()[id].filter(function(candidate) {
        return candidate.indexOf(query) != -1;
      });  // TODO: If this is too slow, swap with binary search
      if (results.length < 1) {
        matchLengths = schemaData.cur()[id].map(function(candidate) {
          var maxMatch = 0;
          for(var start = 0; start < candidate.length; start++) {
            var i = 0;
            for (; start + i < candidate.length && i < query.length; i++) {
              if (candidate[start + i] != query[i]) {
                break;
              }
            }
            if(i > maxMatch) {
              maxMatch = i;
            }
          }
          return maxMatch;
        });
        maxMatch = Math.max.apply(null, matchLengths);
        results = schemaData.cur()[id].filter(function(_, idx) {
          return matchLengths[idx] >= maxMatch;
        });
      }
      $('#' + id + '-results').html(results.map(function(c) {
        if(c.length > 0) {
          return '<option value=' + c + '>' + c + '</option>';
        } else {
          return '<option value=' + c + '>(none)</option>';
        }
      }).join(''));
      var updateStuff = function() {
        // NOTE: Very inefficient right now.
        var lines = plotData.getAvailableLines();
        $('#line-num').html(lines.length + ' lines selected.');
        if (lines.length == 0) return;
        var options = new Array(lines[0].length);
        var keys = schema.getKeys();
        for (var i = 0; i < options.length; i++) {
          options[i] = schemaData.cur()[keys[i]].slice(0);
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
        if (schemaData.cur()[order[i]].indexOf(keys[i]) == -1 &&
            order[i] != 'measurementType') {
          console.log(keys[i]);
          schemaData.cur()[order[i]].push(keys[i]);
        }
      }
    },

    /** Returns the private variables for debugging purposes. */
    debug: function() {
      return schemaData;
    }
  };
})();


function makePlainURL() {
  return [window.location.protocol, '//', window.location.host,
      window.location.pathname].join('');
}

/* Make a tree string from the given parameters
 * This hopefully provides some compression without entirely destroying legibility
 * The tree string looks something like ca..t~n~., which represents ['cat', 'can']
 * Grammar's something like:
 * node = <string> | <prefix> ".." [node "~"]+ "."  */
function makeTree(prefix, strs) {
  if(strs.length == 0) {
    return prefix;
  } else if(strs.length == 1) {
    return prefix + strs[0];
  } else {
    var lastPrefix = '';
    var groups = [];
    var hadEmptyString = strs.some(function(s) {return s.length == 0;});

    // Remove empty strings
    strs = strs.filter(function(s) {return s.length != 0;});
    if(hadEmptyString) {
      groups.push(['']);
    }
    strs.sort();
    // Greedily group strings together, first by the first letter
    strs.forEach(function(str) {
      // Make a new group if the string starts with a different character
      if(lastPrefix.length == 0 || str[0] != lastPrefix) {
        lastPrefix = str[0];
        groups.push([]);
      }
      last(groups).push(str);
    });
    var getLongestMatch = function(strs) {
      var longestMatch = 0;
      var shortestString = Math.min.apply(null, 
          strs.map(function(str) {return str.length;}));
      while(longestMatch < shortestString &&
          strs.every(function(str) {
              return str[longestMatch] == strs[0][longestMatch]})) {
        longestMatch++;
      }
      return longestMatch;
    };
    return prefix + '..' + groups.map(function(group) {
      var longestMatch = getLongestMatch(group);
      var shortenedStrings = group.map(function(str) {
        return str.slice(longestMatch);
      });
      return makeTree(group[0].slice(0, longestMatch), shortenedStrings);
    }).join('~') + '~.';
  }
}


/** Creates a string for the URL using the legend and visible state data.*/
function makeHashString() {
  var legend = getLegendKeys();
  return 'set=' + schema.getCurrentSchema() + '&' +
      'legend=' + encodeURIComponent(makeTree('', legend));
}


/** Unpacks a tree string. Unencoded key go through without change.*/
function unTree(prefix, string) {
  if(string.indexOf('..') == -1 && string.indexOf('~.') == -1) {
    return [prefix + string];
  }
  var newPrefix = string.split('..')[0];
  // Find the  outermost '..' and '.~', 
  // everything inside of that is a child of this one.
  var childrenStr = string.slice(string.indexOf('..') + '..'.length,
      string.lastIndexOf('~.'));
  // Separate out the children by walking the string and finding the '~' that
  // match the outer most layer
  var currentDepth = 0;
  var lastChildEnd = -1;
  var children = [];
  for(var i = 0; i < childrenStr.length; i++) {
    if(childrenStr[i] == '~' && currentDepth == 0) {
      children.push(childrenStr.slice(lastChildEnd+1, i));
      lastChildEnd = i;
    } else if(childrenStr[i] == '.' && childrenStr[i+1] == '.') {
      currentDepth++;
      i++;
    } else if(childrenStr[i] == '~' && childrenStr[i+1] == '.') {
      currentDepth--;
      i++;
    }
  }
  children.push(childrenStr.slice(lastChildEnd+1));
  return children.reduce(function(prev, child) {
    return prev.concat(unTree(prefix + newPrefix, child));
  }, []);
}


/** Updates the page history to match the currently selected lines. */
function updateHistory() {
  var legend = getLegendKeys();
  var historyState = {
    legendState: legend
  };
  window.history.replaceState(historyState, 'foo', 
          makePlainURL() + '#' + makeHashString());
}


/** Adds a page history state to match the change in selected lines. */
function addHistory() {
  var legend = getLegendKeys();
  var historyState = {
    legendState: legend
  };
  console.log(historyState);
  window.history.pushState(historyState, 'foo', 
          makePlainURL() + '#' + makeHashString());
}

// Make the plot
window.addEventListener('load', function() {
  // Load JSON with hash to timestamp conversions.
  // May or may not be needed in the end, but

  var initStuff = function() {
    // Load data from query string
    if(window.location.hash) {
      var processedSearch = window.location.hash.slice(1).split('&');
      processedSearch.forEach(function(str) {
        var name = str.split('=')[0];
        var data = unTree('', decodeURIComponent(str.split('=')[1]));
        if(name == 'legend' && str.split('=')[1] > 0) {
          data.forEach(function(d) { 
            addToLegend(d);
            plotData.makeVisible(d); 
          });
        }
      });
    }
    plotInit();
    if(plotData.getVisibleKeys().length > 0) {
      updateSlidersFromChart(plotData.getProcessedPlotData());
    }
  };

  if(!window.location.hash) {
    plotData.loadData('skps', initStuff);
  } else {
    var targetSet = window.location.hash.split('&').filter(function(line) {
      return line.indexOf('set=') != -1;
    });
    if(targetSet.length > 0) {
      plotData.loadData(targetSet[0].split('=')[1], initStuff);
    }
  }

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

  document.getElementById('add-lines').addEventListener('click', function() {
    console.log('add click');
    plotData.getAvailableLines().forEach(function(ary) {
      plotData.getAndAddLine(ary.join(KEY_DELIMITER), undefined, true);
    });
    plotStuff(plotData);
    addHistory();
    // Clear selections
    schema.updateSchema();
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
      $('#min-zoom-value').val(toRFC(newMin));
      $('#max-zoom-value').val(toRFC(newMax));
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
      var newMin = Date.parse($('#min-zoom-value').val())/1000;
      var newMax = Date.parse($('#max-zoom-value').val())/1000;
      console.log(newMin);
      console.log(newMax);
      if (isNaN(newMin) || isNaN(newMax) ||
          newMin < $('#min-zoom').attr('min') ||
          newMax > $('#max-zoom').attr('max')) {
        console.log('invalid input');
        notifyUser('Invalid input');
        return;
      }
      var realMin = Math.min(newMin, newMax);
      var realMax = Math.max(newMin, newMax);
      console.log(realMin);
      console.log(realMax);
      $('#min-zoom').val(realMin);
      $('#max-zoom').val(realMax);
      updateChartFromSliders();
    }
  };
  document.getElementById('min-zoom-value').
      addEventListener('blur', zoomBlurHandler);
  document.getElementById('max-zoom-value').
      addEventListener('blur', zoomBlurHandler);

  document.getElementById('nuke-plot').addEventListener('click', function(e) {
    console.log('all lines removed');
    while (plotData.getVisibleKeys().length > 0) {
      plotData.makeInvisible(last(plotData.getVisibleKeys()), true);
    }
    plotStuff(plotData);
    $('#legend table tbody').children().remove();
    updateHistory();
    e.preventDefault();
  });

  Array.prototype.forEach.call(document.getElementsByName('schema-type'), 
          function(e) {
    e.addEventListener('change', function() {
      console.log('schema change');
      var newSchema = this.value;
      plotData.loadData(newSchema);
      updateHistory();
    });
  });

  window.addEventListener('popstate', function(event) {
    console.log(event);
    var state = event.state;
    if (state && state.legendState) {
      // TODO: Do this more intelligently.
      var newLegend = state.legendState;
      while (plotData.getVisibleKeys().length > 0) {
        plotData.makeInvisible(last(plotData.getVisibleKeys()));
      }
      $('#legend table tbody').children().remove();
      for (var i = 0; i < newLegend.length; i++) {
        console.log('Adding ' + newLegend[i] + 'to legend');
        addToLegend(newLegend[i]);
        plotData.makeVisible(newLegend[i]);
      }
    }
    plotStuff(plotData);
  });

});
