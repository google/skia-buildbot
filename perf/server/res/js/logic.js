/**
 * Copyright (c) 2014 The Chromium Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style license that can be
 * found in the LICENSE file */
/** Provides the logic behind the performance visualization webpage. */

function assert_(cond, msg) {
  if(!cond) {
    throw msg || "Assertion failed";
  }
}


function last(ary) {
  return ary[ary.length - 1];
}


function getKeys(dict) {
  var keys = [];
  for(var key in dict) {
    if(dict.hasOwnProperty(key)) {
      keys.push(key);
    }
  }
  return keys;
}


function getArg(key) {
  var keypairs = window.location.hash.slice(1).split('&').map(function(p) {
      return p.split('=', 2);});
  for(var i = 0; i < keypairs.length; i++) {
    if(keypairs[i][0] == key && keypairs[i][1].length > 0) {
      return decodeURIComponent(keypairs[i][1]);
    }
  }
  return null;
}


var id = function(e) { return e; };


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

/* Unordered data structure with O(1) add, remove, and index.*/
// Benchmarked remarkably well against vector.splice()/push()
function Vector() {
  var data = [];
  var firstEmpty = 0;
  if(arguments) {
    data = Array.prototype.map.call(arguments, id);
    firstEmpty = data.length;
  }
  this.get = function(idx) {
    assert_(idx < firstEmpty);
    return data[idx];
  },
  this.push = function(elem) {
    data[firstEmpty] = elem;
    firstEmpty++;
  };
  this.pop = function(idx) {
    assert_(idx < firstEmpty && firstEmpty > 0);
    var result = data[idx];
    data[idx] = data[firstEmpty - 1];
    data[firstEmpty - 1] = null;
    firstEmpty--;
    return result;
  };
  this.all = function() {
    return data.slice(0, firstEmpty);
  };
  this.remove = function(elem) {
    var i = 0;
    while(i < firstEmpty && data[i] != elem) { i++; }
    this.pop(i);
  };
  this.map = function() {
    return Array.prototype.map.apply(this.all(), arguments);
  };
  this.has = function(val) {
    for(var i = 0; i < firstEmpty; i++) {
      if(data[i] == val) { return true; }
    }
    return false;
  };
}


function loadJSON(uri, success, fail) {
  var req = new XMLHttpRequest();
  document.body.classList.add('waiting');
  req.addEventListener('load', function() {
    console.log(req);
    if(req.response && req.status == 200) {
      if(req.responseType == 'json') {
        success(req.response);
      } else {
        success(JSON.parse(req.response));
      }
    } else {
      fail();
    }
  });
  req.addEventListener('loadend', function() {
    document.body.classList.remove('waiting');
  });
  req.addEventListener('error', function() {
    notifyUser('Unable to retrieve' + uri);
    fail();
  });
  req.open('GET', uri, true);
  req.send();
}


/** Sends the user a notification on the bottom bar. */
function notifyUser(text, replace) {
  if (!$('#notification').is(':visible') && !replace) {
    $('#notification-text').html(text);
    $('#notification').show().delay(5000).fadeOut(1000).hide(10);
    // If you find a way to convert this to non-jQuery, I'll replace it.
  } else {
    window.setTimeout(notifyUser, 400, text);
  }
}


/** Splits a particular component out of a list of objects.*/
function getComponent(ary, name) {
  return ary.map(function(e) { return e[name]; });
}


/** Removes all the children owned by the element.*/
function killChildren(e) {
  while(e.hasChildNodes()) {
    e.removeChild(e.children[0]);
  }
}


// FUTURE(kelvinly): Try using a dirty flag instead of two representations
// to see if it's more efficient.
/** Encapsulates the legend state.*/
var legend = (function() {
  var internalLegend = [];
  var externalLegend = [];
  // Each legend marker has two values, a key and a color
  // The external legend also has a pointer to its DOM element called elem.
  var legendBody = null;

  function addToDOM(e) {
    console.log('addToDOM called');
    // console.log(e);
    assert_(e.key && e.color);
    assert_(legendBody);
    var container = document.createElement('tr');
    var checkContainer = document.createElement('td');
    var checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = true;
    checkbox.id = e.key;
    var outerColor = document.createElement('div');
    outerColor.classList.add('legend-box-outer');
    var innerColor = document.createElement('div');
    innerColor.classList.add('legend-box-inner');
    innerColor.style.border = '5px solid ' + e.color;
    var text = document.createTextNode(e.key);
    var linkContainer = document.createElement('td');
    var link = document.createElement('a');
    link.href = '#';
    link.innerText = 'Remove';
    link.id = e.key + '_remove';
    
    container.appendChild(checkContainer);
    container.appendChild(linkContainer);
    
    checkContainer.appendChild(checkbox);
    checkContainer.appendChild(outerColor);
    checkContainer.appendChild(text);

    linkContainer.appendChild(link);
    outerColor.appendChild(innerColor);

    legendBody.appendChild(container);
    return {key: e.key, color: e.color, elem: container};
  }

  function rawAdd(key, color) {
    assert_(getComponent(internalLegend, 'key').indexOf(key) == -1);

    var newPair = {key:key, color:color};
    internalLegend.push(newPair);
    externalLegend.push(addToDOM(newPair));
  }
  return {
    /** Updates the colors for the elements.*/
    updateColors: function(plotRef) {
      var dataRef = plotRef.getData();
      internalLegend.forEach(function(legendMarker) {
        legendMarker.color = legendMarker.color || 'white';
        dataRef.forEach(function(series) {
          if(legendMarker.key == series.label) {
            legendMarker.color = series.color;
          }
        });
      });
    },
    /** Refreshes the DOM to match the internal state.*/
    refresh: function() {
      var synced, color_synced = true;
      synced = internalLegend.length == externalLegend.length;
      // NOTE: Restructure control flow?
      internalLegend.forEach(function(elem, idx) {
        if(!synced) {
          return;
        } else {
          if(elem.key != externalLegend[idx].key) {
            synced = false;
            color_synced = false;
          }
          if(elem.color != externalLegend[idx].color) {
            color_synced = false;
          }
        }
      });
      if(synced && color_synced) {
        console.log('legend.refresh: legends synced');
        return;
      } else if(synced && !color_synced) {
        console.log('legend.refresh: fixing colors');
        // Fix the colors
        $$('tr', legendBody).forEach(function(e, idx) {
          assert_($$$('input', e).id == internalLegend[idx].key);

          $$$('.legend-box-inner', e).style.
            border = '5px solid ' + internalLegend[idx].color;
        });
      } else {
        killChildren($$$('#legend table tbody'));
        externalLegend = [];
        // Regenerate a new legend
        var _this = this;
        internalLegend.forEach(function(e) {
          externalLegend.push(addToDOM(e, _this));
        });
      }
    },
    remove: function(key) {
      var children = [];
      externalLegend.forEach(function(e, idx) {
        if(e.key == key) {
          children.push(e.elem);
        }
      });
      children.forEach(function(c) {
        assert_(c.parentNode);
        c.parentNode.removeChild(c);
      });
      internalLegend = internalLegend.filter(function(e) {
        return e.key != key;
      });
      externalLegend = externalLegend.filter(function(e) {
        return e.key != key;
      });
    },
    /* Sets up the private variables, and a few of the relevant UI controls.*/
    init: function(showHandler, hideHandler, drawHandler) {
      console.log('Initializing legend');
      assert_(showHandler && hideHandler);
      legendBody = $$$('#legend table tbody');
      var _this = this;
      $$$('#nuke-plot').addEventListener('click', function(e) {
        internalLegend.forEach(function(keypair) {
          plotData.hide(keypair.key);
        });
        killChildren($$$('#legend table tbody'));
        internalLegend = [];
        externalLegend = [];
        drawHandler();
        e.preventDefault();
      });
      legendBody.addEventListener('click', function(e) {
        if('INPUT' == e.target.nodeName) {
          if(e.target.checked) {
            console.log(e.target.id + ' checked');
            showHandler(e.target.id);
          } else {
            console.log(e.target.id + ' unchecked');
            hideHandler(e.target.id);
          }
          drawHandler();
        } else if('A' == e.target.nodeName) {
          console.log(e.target.id + ' removed');
          if(document.getElementById(e.target.id.slice(0,-'_remove'.length)).checked) {
            hideHandler(e.target.id.slice(0,-'_remove'.length));
          }
          _this.remove(e.target.id.slice(0,-'_remove'.length));
          drawHandler();
        }
      });
      /* Adds a key to the legend, and makes it visible on the chart.*/
      this.add = function(key, color, nodraw) {
        if(getComponent(internalLegend, 'key').indexOf(key) != -1) {
          return;
        }
        rawAdd(key, color);
        showHandler(key);
        if(!nodraw) {
          drawHandler();
        }
      };
      /* Adds an array of keys to the legend, and makes them visible 
       * on the chart.*/
      this.addMany = function(ary) {
        ary.forEach(function(a) { _this.add(a.key, a.color, true); });
        drawHandler();
      };
    },
    debug: function() {
      return [internalLegend, externalLegend, legendBody];
    }
  };
})();


/** Attempts to merge requests for the same resource, waits 50 ms before sending
 * a request */
var jsonRequest = (function() {
  var waitingHandlers = [];
  var freshFiles = [];

  function makeRequest(uri, callback) {
    var ref = {uri: uri, callbacks: [callback]};
    waitingHandlers.push(ref);

    var removeSelf = function() {
      var idx = waitingHandlers.indexOf(ref);
      assert_(idx != -1);
      waitingHandlers.splice(idx, 1);
    };

    loadJSON(ref.uri, function(data) {
      console.log('jsonRequest: ' + ref.uri + ' received');
      var freshIdx = getComponent(freshFiles, 'uri').indexOf(uri);
      if(freshIdx == -1) {
        freshFiles.push({uri: uri, time: Date.now(), data: data});
      } else {
        freshFiles[freshIdx].data = data;
        freshFiles[freshIdx].time = Date.now();
      }
      ref.callbacks.forEach(function(callback) {
        assert_(callback);
        callback(data, true);
      });
      removeSelf();
    }, function() {
      ref.callbacks.forEach(function(callback) {
        assert_(callback);
        callback(data, false);
      });
      removeSelf();
    });
  }
  return {
    askFor: function(uri, callback) {
      var idx = getComponent(waitingHandlers, 'uri').indexOf(uri);
      var freshIdx = getComponent(freshFiles, 'uri').indexOf(uri);
      if(idx != -1) {
        // Add to to that handler's callbacks.
        waitingHandlers[idx].callbacks.push(callback);
      } else if(freshIdx != -1) {
        if(Date.now() - freshFiles[freshIdx].time < 5*60*1000) {
          // Good enough
          callback(freshFiles[freshIdx].data, true);
          return;
        } else {
          // Not fresh enough any more
          freshFiles.splice(freshIdx, 1);
          makeRequest(uri, callback);
        }
      } else {
        makeRequest(uri, callback);
      }
    },
    forceReload: function(uri, callback) {
      var freshIdx = getComponent(freshFiles, 'uri').indexOf(uri);
      while(freshIdx != -1) {
        freshFiles.splice(freshIdx, 1);
        freshIdx = getComponent(freshFiles, 'uri').indexOf(uri);
      }
      makeRequest(uri, callback);
    },
    askForTile: function(scale, tileNumber, dataset, 
            options, callback, forcerefresh) {
      if(!forcerefresh) {
        // FUTURE: Use other passed in data
        this.askFor('json/' + dataset /* + makeArgs(options) */, callback);
      } else {
        // FUTURE: Use other passed in data
        this.forceRefresh('json/' + dataset /* + makeArgs(options) */, callback);
      }
    },
    debug: function() {
      return [freshFiles, waitingHandlers];
    }
  };
})();


// Stores a set of traces for a single key, over all tiles and ranges. The
// constuctor takes in a set of data to start it off.
function Trace(newData, newTileID, newScale) {
  var data = [];
  data[parseInt(newScale)] = [];
  data[parseInt(newScale)][parseInt(newTileID)] = newData.slice(); 
  // Will replace with shallow copy if not performant
  // FUTURE: Assumes input data is sorted

  function getscales() {
    var result = [];
    for(var key in data) {
      if(data.hasOwnProperty(key)) {
        result.push(parseInt(key));
      }
    }
    return result;
  }

  function getTiles(scale) {
    if(!data[scale]) {
      return [];
    }
    var result = [];
    for(var key in data[scale]) {
      if(data[scale].hasOwnProperty(key)) {
        result.push(parseInt(key));
      }
    }
    return result;
  }

  /* Adds a set of data to the trace.*/
  this.add = function(newData, tileId, scale) {
    assert_(newData.length > 0);
    if(!data[scale]) {
      data[scale] = [];
    }
    if(!data[scale][tileId] || (newData.length >= data[scale][tileId].length)) {
      data[scale][tileId] = newData.slice();  // FUTURE: If too slow, replace with
                                              // shallow copy
    }
  };

  this.get = function(tileId, scale) {
    return (data[scale] && data[scale][tileId]) || [];
  };

  this.getRange = function(start, end, scale) {
    // FUTURE: Add support for downsampling on scale mismatch
    assert_(start <= end);
    var results = [];
    var tiles = data[scale];
    getTiles(scale).forEach(function(tileIdx) {
      if(tiles[tileIdx][0][0] <= end && last(tiles[tileIdx])[0] >= start) {
        var result = [];
        var i = 0;
        while(tiles[tileIdx][i][0] < start) {
          assert_(i < tiles[tileIdx].length);
          i++;
        }
        if(i > 0) {
          // Add one just before the range if possible
          i--;
        }
        while(i < tiles[tileIdx].length && tiles[tileIdx][i][0] <= end) {
          result.push(tiles[tileIdx][i]);
          i++;
        }
        if(i < tiles[tileIdx].length) {
          // Also add one just after
          result.push(tiles[tileIdx][i]);
        }
        results.push(result);
      }
    });
    return results;
  }

  /* Returns true if the trace has data in that tile and scale.*/
  this.contains = function(tileid, scale) {
    return !!(data[scale] && data[scale][tileid]);
  };

  this.debug = function() {
    return data;
  };
}


var traceDict = (function() {
  var cache = new PagedDictionary();
  // A dictionary of keys to Trace objects

  function loadData(data, tileId, dataset, scale) {
    console.log('traceDict: loadData called. tileId = ' + tileId + 
        ', scale = ' + scale);
    // Look for the key in the data, and store that. If no key specified, store
    // as much data as possible.
    assert_(data['traces'] && data['commits']);

    var commitAry = getComponent(data['commits'], 'commit_time');
    data['traces'].forEach(function(trace) {
      var newKey = schema.makeLegendKey(trace);
      var processedData = [];
      for(var i = 0; i < trace['values'].length; i++) {
        if(trace['values'][i] < 1e+99) {
          processedData.push([commitAry[i], trace['values'][i]]);
        }
      }
      if(cache.has(newKey)) {
        cache.get(newKey).add(processedData, tileId, scale);
      } else {
        cache.add(newKey, new Trace(processedData, tileId, scale));
      }
    });
  }
  return {
    getTraces: function(toGet, callback) {
      var result = {};
      var count = toGet.length;
      var writeResults = function(key, data) {
        result[key] = data;
        count--;
        if(count <= 0) {
          callback(result);
        }
      };
      var failResults = function() {
        count--;
        if(count <= 0) {
          callback(result);
        }
      };
      toGet.forEach(function(metadata) {
        // NOTE: Assumes getTileNumbers returns in numeric order
        commitDict.getTileNumbers(metadata.range, function(tileData) {
          var tileNums = tileData[0];
          var scale = tileData[1];
          var tileCount = tileNums.length;
          var tileData = [];
          var writeSegment = function(tileId, data) {
            tileData.push.apply(tileData, data);
            tileCount--;
            if(tileCount <= 0) {
              writeResults(metadata.key, tileData);
            }
          };
          var writeFromCache = function(tileId) {
            writeSegment(tileId, cache.get(metadata.key).get(tileId, scale));
          };
          tileNums.forEach(function(tileId) {
            if(cache.has(metadata.key) && 
                cache.get(metadata.key).contains(tileId, scale)) {
              //console.log('trace segment ' + metadata.key + ':' +
                  //tileId + ':' + scale + ' found in cache');
              writeFromCache(tileId);
            } else {
              //console.log('traceDict.getTraces: line ' + metadata.key + ' not cached.');
              var dataset = metadata.key.split(':')[0];
              jsonRequest.askForTile(scale, tileId, dataset, {} /* individual trace set here */,
                  function(data) {
                loadData(data, tileId, dataset, scale);
                // FUTURE: Check to see if this causes race conditions?
                if(cache.has(metadata.key) && 
                    cache.get(metadata.key).contains(tileId, scale)) {
                  //console.log('trace segement ' + metadata.key + ':' + 
                      //tileId + ':' + scale + ' found after loading');
                  writeFromCache(tileId);
                } else {
                  tileCount--;
                  console.log('This line appears to not be in selected tile.');
                }
              });
            }
          });
        });
      });
    },
    /* Returns the number of traces that are cached from the array of lines
     * passed in.*/
    countCached: function(keyArray) {
      var count = 0;
      keyArray.forEach(function(key) {
        if(cache.has(key)) { count ++; }
      });
      return count;
    },
    init: function() {
      console.log('Initializing traceDict');
      // Nothing here; data's loaded when requested
    },
    debug: function() {
      return cache;
    }
  };
})();


var commitDict = (function() {
  var dataDict = new PagedDictionary();
  // Uses timestamp plus scale as keys, {hash, commit_msg, blamelist}, etc as values
  var callbacks = new Vector();

  /* Calls a function for each non empty element of callbackentries.
   * If the function returns true, then after iterating, remove that
   * element. */
  function iterateAndPopCallback(fun) {
    var toRemove = callbacks.map(function(e, idx) {
      if(fun(e)) {
        return idx;
      } else {
        return null;
      }
    }).filter(function(e) {return e != null;});
    // Iterate in reverse, since removing an element will only disturb the
    // ones with a higher index than it
    for(var i = toRemove.length - 1; i >= 0; i--) {
      callbacks.pop(toRemove[i]);
    }
  }

  return {
    /* Gets all the data associated with a timestamp.*/
    getAssociatedData: function(timestamp, scale, callback) {
      if(dataDict.has(scale) && dataDict.get(scale).has(timestamp)) {
        return dataDict.get(scale).get(timestamp);
      } else if(callback) {
        callbacks.push({lookup: timestamp, scale: scale, callback: callback});
        return null;
      }
    },
    /* Call the callback with the hash for the given timestamp.*/
    timestampToHash: function(timestamp, scale, callback) {
      var res = this.getAssociatedData(timestamp, scale, function(res) {
        assert_(res && res.hash);
        callback(res.hash);
      });
      return res && res.hash;
    },
    /* Updates the dictionaries with the JSON data.*/
    update: function(data, isManifest) {
      console.log('commitDict.update called: isManifest=' + isManifest);
      console.log(data);
      // Load new data, then see if any of the callbacks are now valid
      if(isManifest) {
        // it's a manifest JSON
        // FUTURE
        assert_(false, "Unimplemented");
      } else {
        // it's a tile JSON
        // FUTURE: Remove hack when the JSON has the right format
        data['scale'] = data['scale'] || 0;

        assert_(data && data['commits']); // FUTURE: add: && data['scale']);
        var commits = data['commits'];
        var scale = 0; // FUTURE: Replace with: parseInt(data['scale']);
        commits.forEach(function(commit) {
          assert_(commit['commit_time']);
          if(!dataDict.has(scale)) {
            dataDict.add(scale, new PagedDictionary());
          }
          if(!dataDict.get(scale).has(commit['commit_time'])) {
            dataDict.get(scale).add(parseInt(commit['commit_time']), commit);
          }
        });
      }
      iterateAndPopCallback(function(entry) {
        assert_(entry.callback);
        if(callbackObject.hasOwnProperty('lookup')) { // Then it's a timestamp look up
          var res = getAssociatedData(entry.timestamp, entry.scale, null);
          if(res != null) {
            entry.callback(res);
          }
          return res != null; // Remove the entry if the get was successful
        }
        return false;
      });
    },
    /* Looks only through the available commit data, returning the array
     * of values foundIt returns true on.*/
    lazySearch: function(foundIt) {
      var searchSpace = dataDict.index();
      var results = [];
      searchSpace.forEach(function(key) {
        var maybeResult = dataDict.get(key);
        if(foundIt(maybeResult)) {
          results.push(maybeResult);
        }
      });
      return results;
    },
    /* It'll call the callback when it can pass the tile numbers and scale
     * for the range.*/
    getTileNumbers: function(range, callback) {
      // FUTURE: Actually make work when we have tiles
      callback([[0], 0]);
    },
    /* Called on start up.*/
    init: function() {
      console.log('Initializing commitDict');
      var _this = this;
      // FUTURE: Also load manifest
      jsonRequest.askForTile(0, -1, 'skps', {'use_commit_data': true}, 
          function(data, success) {
            if(success) {
              _this.update(data);
            } else {
              console.log('traceDict.init failed to retrieve data');
            }
          });
    },
    debug: function() {
      return [dataDict, callbacks];
    }
  };
})();


var schema = (function() {
  var KEY_DELIMITER = ':';
  var currentDataset;
  var schemaDict = new PagedDictionary(); 
  var hiddenChildren = new PagedDictionary();
  // Contains a dictionary of dictionary of config key-values
  var validKeyParts = {
    'micro': ['dataset', 'builderName', 'system', 'testName', 'gpuConfig',
        'measurementType'],
    'skps': ['dataset', 'builderName', 'benchName', 'config',
        'scale', 'measurementType']
  };
  var keysWithEmpty = ['config'];
  var lineList = [];
  // List of all lines keys

  // Makes sure the children of root match the given model
  // Model has the format
  // [
  //   { 
  //      nodeType: <string>,   // Required
  //      id: <string>,
  //      style: { ... },
  //      attributes: { ... },
  //      text: <string>,
  //      children: [models]
  //   }
  // ]
  function diffReplace(root, model) {
    var checkSet = function(obj, fieldName, value) {
      if(obj[fieldName] != value) {
        obj[fieldName] = value;
      }
    }
    var checkSetAttr = function(node, fieldName, value) {
      if(node.getAttribute(fieldName) != value) {
        node.setAttribute(fieldName, value);
      }
    }
    var specialCases = ['children', 'text', 'attributes', 'style'];
    for(var i = 0; i < model.length; i++) {
      assert_(model[i].nodeType);
      if(i >= root.children.length || 
          root.children[i].nodeName != model[i].nodeType.toUpperCase()) {
        root.appendChild(document.createElement(model[i].nodeType));
      }
      var curChild = root.children[i];
      for(var name in model[i]) {
        if(model[i].hasOwnProperty(name) && specialCases.indexOf(name) == -1) {
          checkSet(curChild, name, model[i][name]);
        }
      }
      if(model[i].text) { checkSet(curChild, 'innerText', model[i].text); }
      for(var styleName in model[i].style) {
        if(model[i].style.hasOwnProperty(styleName)) {
          checkSet(curChild.style, styleName, model[i].style[styleName]);
        }
      }
      for(var attrName in model[i].attributes) {
        if(model[i].attributes.hasOwnProperty(attrName)) {
          checkSetAttr(curChild, attrName, model[i].attributes[attrName]);
        }
      }
      if(model[i].children) {
        diffReplace(curChild, model[i].children);
      }
    }
    // Get rid of extras
    while(root.length > model.length) {
      root.removeChild(last(root.children));
    }
  }

  function getOptions() {
    var keyParts = $$('#line-table select');
    var options = {};
    keyParts.forEach(function(keyPart) {
      partName = keyPart.id.slice(0, -'-results'.length);
      $$('option:checked', keyPart).forEach(function(selectedOption) {
        if(!options[partName]) { options[partName] = []; }
        options[partName].push(selectedOption.value);
      });
    });
    options['dataset'] = currentDataset;
    return options;
  }

  return {
    /* Returns a string given the key elements in trace.*/
    makeLegendKey: function(trace, dataset) {
      assert_(trace['params']);
      if(!dataset) {
        assert_(trace['params']['dataset']);
        dataset = trace['params']['dataset'];
      } else if(!trace['params']['dataset']) {
        trace['params']['dataset'] = dataset;
      }
      assert_(validKeyParts[dataset]);
      return validKeyParts[dataset].map(function(part) {
        return trace['params'][part] || '';
      }).join(KEY_DELIMITER);
    },
    /* Updates the schema given data in the JSON input.*/
    update: function(data, datasetName) {
      console.log('schema.update called: datasetName=' + datasetName);
      console.log(data);
      assert_(data['param_set']);
      // Update internal structure
      if(!schemaDict.has(datasetName)) {
        schemaDict.add(datasetName, new PagedDictionary());
      }
      var keys = [];
      for(var key in data['param_set']) {
        if(data['param_set'].hasOwnProperty(key) && 
            validKeyParts[datasetName].indexOf(key) != -1) {
          keys.push(key);
        }
      }
      keys.forEach(function(key) {
        if(schemaDict.get(datasetName).has(key)) {
          var newParams = data['param_set'][key].filter(function(param) {
            return schemaDict.get(datasetName).get(key).indexOf(param) == -1;
          });
          schemaDict.get(datasetName).get(key).push.apply(newParams);
        } else {
          schemaDict.get(datasetName).add(key, data['param_set'][key]);
        }
        if(keysWithEmpty.indexOf(key) != -1 && 
            schemaDict.get(datasetName).get(key).indexOf('') == -1) {
          schemaDict.get(datasetName).get(key).push('');
        }
      });

      if(data['traces']) {
        data['traces'].forEach(function(trace) {
          lineList.push(this.makeLegendKey(trace, datasetName));
        }, this);
      }
      this.updateDOM();
    },
    /* Given the input options, returns the ones that define real traces and
     * their number.
     * options is a dictionary keyed with the key part name and has the
     * selected values as its value.*/
    getValidOptions: function(options, dataset) {
      // FUTURE: Replace with tree if not performing well enough
      assert_(validKeyParts[dataset]);
      var mapOptions = function(key) {
        // Return the split string if it's valid, false otherwise.
        var parts = key.split(KEY_DELIMITER);
        return key.split(KEY_DELIMITER).every(function(part, idx) {
          return !options[validKeyParts[dataset][idx]] ||
              options[validKeyParts[dataset][idx]].indexOf(part) != -1;
        }) && parts;
      };
      var validLines = lineList.map(mapOptions);
      var optionDicts = {};
      var count = 0;
      // Get all the valid options
      for(var i = 1; i < validKeyParts.length; i++) {
        var tmpDict = {};
        for(var j = 0; j < validLines.length; j++) {
          if(validLines) {
            //O(1) set addition! There's a little latency on the microbench
            // options bar, hopefully this helps with that..
            tmpDict[validLines[j][i]] = true;
            count++;
          }
        }
        var tmpOptions = [];
        for(var k in tmpDict) {
          if(tmpDict.hasOwnProperty(k)) {
            tmpOptions.push(k);
          }
        }
        optionDicts[validKeyParts[i]] = tmpOptions;
      }
      return [optionDicts, count];
    },
    /* Returns a list of valid lines given the selected options.*/
    getValidLines: function(options, dataset) {
      // FUTURE: Replace with tree if not performing well enough
      assert_(validKeyParts[dataset]);
      var mapOptions = function(key) {
        // Return the split string if it's valid, false otherwise.
        var parts = key.split(KEY_DELIMITER);
        return parts.every(function(part, idx) {
          return !options[validKeyParts[dataset][idx]] ||
              options[validKeyParts[dataset][idx]].indexOf(part) != -1;
        }) && key;
      };
      var validLines = lineList.map(mapOptions);
      // console.log(validLines);
      return validLines.filter(id);
    },
    /* Updates the selection boxes to match the ones currently in the schema.*/
    updateDOM: function() {
      assert_(currentDataset);
      console.log('schema.updateDOM: start');
      if(!schemaDict.has(currentDataset)) {
        console.log('schema.updateDOM: Schema for selected dataset not ' +
            'currently loaded; sending request.');
        var _this = this;
        jsonRequest.askForTile(0, -1, currentDataset, {'get_params_data': true},
            function(data, success) {
              if(success) {
                console.log('schema.updateDOM: received data.');
                _this.update(data, currentDataset);
              }
            });
        return;
      }
      assert_($$$('#line-table'));
      var inputRoot;
      if($$$('#' + currentDataset + '-set')) {
        inputRoot = $$$('#' + currentDataset + '-set');
        assert_(inputRoot.parentElement == $$$('#line-table'));
      } else {
        inputRoot = document.createElement('tr');
        inputRoot.id = currentDataset + '-set';
      }
      var curDict = schemaDict.get(currentDataset);
      curDict.index().forEach(function(part) {
        curDict.get(part).sort();
      });
      var getWidth = function(part) {
        var longestLine = Math.max.apply(null, 
            curDict.get(part).map(function(names) {
                return names.length;
            }));
        return 0.75 * longestLine + 0.5;
      }
      var selectedValues = {};
      $$('select', inputRoot).forEach(function(opt) {
        var partName = opt.id.slice(0, -'-results'.length);
        selectedValues[partName] = {};
        $$('option', opt).forEach(function(maybeSelected) {
          if(maybeSelected.selected) {
            selectedValues[partName][maybeSelected.value] = true;
          }
        });
      });
      var makeSelectModel = function(part) {
        return curDict.get(part).map(function(option) {
          return {
            nodeType: 'option',
            value: option,
            text: option.length > 0 ? option : '(none)',
            selected: !!(selectedValues[part] && selectedValues[part][option])
          };
        });
      };
      diffReplace(inputRoot, validKeyParts[currentDataset].slice(1).map(
            function(part) {
        return {
          nodeType: 'td',
          children: [
            {
              nodeType: 'input',
              id: part + '-input',
              style: {
                width: getWidth(part) + 'em'
              }
            },
            {
              nodeType: 'select',
              id: part + '-results',
              attributes: {
                multiple: 'yes'
              },
              style: {
                width: getWidth(part) + 'em',
                overflow: 'auto'
              },
              children: makeSelectModel(part)
            }
          ]
        };
      }));
      // Hide the other ones
      $$('tr', $$$('#line-table')).forEach(function(e) {
        e.style.display = 'none';
      });
      $$$('#line-table').appendChild(inputRoot);
      inputRoot.style.display = '';
    },

    /* Greys out elements as needed. Doesn't grey out any more in the currentRow.*/
    updateDisabledDOM: function(currentRow) {
      // Currently unimplemented because it seems like people found it confusing
      // TODO: Fix greyed out areas
    },

    /* Grabs all the possible line names.*/
    init: function() {
      console.log('Initializing schema');
      // Load line metadata, update client controls
      currentDataset = getArg('set') || 'skps';
      jsonRequest.askForTile(0, -1, currentDataset, {include_commit_data: true},
          function(data, success) {
            if(success) {_this.update(data, currentDataset);}
          });
      var _this = this;
      var updateLineCount = function() {
          var lines = _this.getValidLines(getOptions(), currentDataset);
          var count = lines.length;
          var cacheCount = traceDict.countCached(lines);
          $$$('#line-num').innerHTML = count + ' valid lines, ' + 
              cacheCount + ' cached lines';
      };
      $$('input[name=\'schema-type\']').forEach(function(e) {
        console.log('schema: Adding event listener for ' + e.value);
        console.log(e);
        if(e.value == currentDataset) { e.checked = true; }
        e.addEventListener('change', function() {
          console.log('schema change');
          var newSchema = this.value;
          console.log(newSchema);
          currentDataset = newSchema;
          _this.updateDOM();
          updateLineCount();
        });
      });
      $$$('#add-lines').addEventListener('click', function(e) {
        // Find relevant lines, add to legend and plot
        var options = getOptions();
        var lines = _this.getValidLines(options, currentDataset).map(function(l) {
          return {key: l, color: 'white'};
        });
        legend.addMany(lines);
        // console.log(lines);
        /*
        lines.forEach(function(line) {
          legend.add(line, 'white');
        });
        */
        plotData.data(plotData.makePlotCallback());
      });
      // Attach event listeners to parent of all schema nodes
      $$$('#line-table').addEventListener('input', function(e) {
        console.log('line-table: input event listener called');
        // console.log(e);
        var inputId = e.target.id.slice(0,-'-input'.length);
        console.log('called for ' + e.target.id);
        if(e.target.nodeName == 'INPUT') {
          if(!currentDataset) {
            console.log('line-table.input: no schema currently loaded. Ignoring.');
            return;
          }
          var query = e.target.value;
          var dataset = schemaDict.get(currentDataset).get(inputId);
          assert_(dataset != null);
          var results = dataset.filter(function(candidate) {
            return candidate.indexOf(query) != -1;
          });  // FUTURE: If this is too slow, swap with binary search
          if (results.length < 1) {
            matchLengths = dataset.map(function(candidate) {
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
            results = dataset.filter(function(_, idx) {
              return matchLengths[idx] >= maxMatch;
            });
          }
          console.log('search results: ');
          console.log(results);
          if(!hiddenChildren.has(currentDataset)) {
            hiddenChildren.add(currentDataset, new PagedDictionary());
          }
          if(!hiddenChildren.get(currentDataset).has(inputId)) {
            hiddenChildren.get(currentDataset).add(inputId, []);
          }
          var hiddenResults = hiddenChildren.get(currentDataset).
              get(inputId);
          var removed = [];
          // Insert appropriate nodes back into the array
          hiddenResults.forEach(function(e, idx) {
            if(results.indexOf(e.value) != -1) {
              var resultsChildren = $$$('#' + inputId + '-results').children;
              for(var i = 0; i < resultsChildren.length; i++) {
                if(resultsChildren[i].value > e.value) {
                  $$$('#' + inputId + '-results').insertBefore(
                    e, resultsChildren[i]);
                  removed.push(idx);
                  return;
                }
              }
              $$$('#' + inputId + '-results').insertBefore(e, null);
              removed.push(idx);
            }
          });
          for(var i = removed.length - 1; i >= 0; i--) {
            hiddenResults.splice(removed[i], 1);
          }
          $$('#' + inputId + '-results option').forEach(function(e) {
            if(results.indexOf(e.value) != -1) {
              e.style.display = '';
            } else {
              hiddenResults.push(e);
              e.parentNode.removeChild(e);
            }
          });
          console.log('hidden nodes:');
          console.log(hiddenResults);
        }
      });
      $$$('#line-table').addEventListener('click', function(e) {
        console.log('#line-table.click called');
        //console.log(e);
        if(e.target.nodeName == 'OPTION') {
          console.log('#line-table select.click event listener called');
          // Update the line count
          updateLineCount();
        }
      });
    },
    debug: function() {
      return [schemaDict, currentDataset, lineList];
    }
  };
})();


var history = (function() {
  // TODO: Re-add history features, etc.
})();


/** Returns the datetime compatible version of a POSIX timestamp.*/
function toRFC(timestamp) {
  // Slice off the ending 'Z'
  return new Date(timestamp*1000).toISOString().slice(0, -1);
}


var plotData = (function() {
  var plotRef;
  var isLogPlot;
  var visibleKeys = new Vector();
  function getVisibleKeys() {
    return visibleKeys.all();
  }
  function toZoomValue(min, max) {
    return 60*60/(max - min);
  }
  function fromZoomValue(curMin, curMax, zoomValue) {
    if(zoomValue < 0.005) {
      zoomValue = 0.005;
    }
    var range = 60*60/zoomValue;
    var midpoint = 0.5*(curMin + curMax);
    return [midpoint - range/2, midpoint + range/2];
  }
  function plotClickHandler(evt, pos, item) {
    var notePad = $('#note');
    if(!item) {
      notePad.hide();
      return;
    }
    notePad.hide();
    notePad.css({'top': item.pageY + 10, 'left': item.pageX});
    commitDict.getTileNumbers(getCurRange(), function(tileData) {
      var postNote = function(commitData) {
        console.log(commitData);
        var hashMsg = '';
        var commitMsg = '';
        var authorMsg = '';
        if(commitData['hash']) {
          var hash = commitData['hash'];
          hashMsg = 'hash: <a href=https://github.com/google/skia/commit/' + 
            hash + ' target=_blank>' + hash + '</a><br />';
        }
        if(commitData['commit_msg']) {
          commitMsg = 'commit message: ' + commitData['commit_msg'] + 
              '<br />';
        }
        if(commitData['author']) {
          authorMsg = 'author: ' + commitData['author'] + '<br />';
        }
        $$('#note #data')[0].innerHTML = (
            hashMsg +
            'timestamp: ' + item.datapoint[0] + '<br />' +
            'value: ' + item.datapoint[1] + '<br />' +
            authorMsg + commitMsg
            );
        notePad.show();
      };
      var relatedData = commitDict.getAssociatedData(
          item.datapoint[0], tileData[1], postNote);
      if(relatedData) {
        postNote(relatedData);
      }
    });
  }
  function plotZoomHandler(evt, pos, item) {
    var range = getCurRange();
    $$$('#zoom').value = toZoomValue(range[0], range[1]);
    $$$('#start').value = toRFC(range[0]);
    $$$('#end').value = toRFC(range[1]);
    // TODO: Get more plot data as needed; see if the new range
    // requires new tiles, and if so call data(makePlotCallback())
  }
  function plotPanHandler(evt, pos, item) {
    var range = getCurRange();
    $$$('#start').value = toRFC(range[0]);
    $$$('#end').value = toRFC(range[1]);
    // TODO: Get more plot data as needed; see if the new range
    // requires new tiles, and if so call data(makePlotCallback())
  }
  /* Returns the current visible range. Null to load the latest tile.*/
  function getCurRange() {
    var data = plotRef.getData();
    var xaxis = plotRef.getOptions().xaxes[0];
    var min = null;
    var max = null;
    if(xaxis.min != null && xaxis.max != null) {
      min = xaxis.min;
      max = xaxis.max;
    } else if(plotRef.getData().length > 0) {
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
    return [min, max];
  }

  return {
    /* Initializes the plot.*/
    init: function() {
      console.log('Initializing plotData');
      isLogPlot = false;
      plotRef = $('#chart').plot([],
        {
          legend: {
            show: false
          },
          grid: {
            hoverable: true,
            autoHighlight: true,
            mouseActiveRadius: 16,
            clickable: true,
            markings: this.getMarkings
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
                  // FUTURE: Find a way to make a string with only the hour or minute
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
          }
        }).data('plot');
      $('#chart').bind('plotclick', plotClickHandler);
      $('#chart').bind('plotzoom', plotZoomHandler);
      $('#chart').bind('plotpan', plotPanHandler);

      // Initialize zoom ranges
      $$$('#zoom').setAttribute('min', 0);
      $$$('#zoom').setAttribute('max', 1);
      $$$('#zoom').setAttribute('step', 0.001);

      // Zoom binding
      $$$('#zoom').addEventListener('input', function() {
        var curRange = getCurRange();
        var newRange = fromZoomValue(curRange[0], curRange[1],
            $$$('#zoom').value);
        var xaxis = plotRef.getOptions().xaxes[0];
        xaxis.min = newRange[0];
        xaxis.max = newRange[1];
        plotRef.setupGrid();
        plotRef.draw();
      });

      // Set up go button binding
      $$$('#back-to-the-future').addEventListener('click', function(e) {
        var newMin = Date.parse($$$('#start').value)/1000;
        var newMax = Date.parse($$$('#end').value)/1000;
        if(isNaN(newMin) || isNaN(newMax)) {
          console.log('#back-to-the-future.click: invalid input');
        } else {
          var realMin = Math.min(newMin, newMax);
          var realMax = Math.max(newMin, newMax);
          var xaxis = plotRef.getOptions().xaxes[0];
          xaxis.min = realMin;
          xaxis.max = realMax;
          plotRef.setupGrid();
          plotRef.draw();
        }
      });

      $$$('#islog').addEventListener('click', function(e) {
        var willLogPlot = $$$('#islog').checked;
        if(isLogPlot != willLogPlot) {
          isLogPlot = willLogPlot;
          plotRef.setupGrid();
          plotRef.draw();
        }
      });
    },
    /* Returns usable plot data.*/
    data: function(callback) {
      var processAndCall = function(dataDict) {
        console.log('data.get callback:');
        console.log(dataDict);
        var results = [];
        for(var trace in dataDict) {
          if(dataDict.hasOwnProperty(trace)) {
            // Convert data to Flot readable format
            var curTrace = dataDict[trace];
            results.push({
              label: trace,
              data: curTrace
            });
          }
        }
        //console.log(results);
        callback(results);
      };
      //console.log('visible keys: ');
      //console.log(visibleKeys.all());
      var currentRange = getCurRange();
      // Process keys before passing to traceDict
      var processedKeys = visibleKeys.all().map(function(key) {
        return {
          key: key,
          range: currentRange
        };
      });
      traceDict.getTraces(processedKeys, processAndCall);
    },
    getMarkings: function() {
      if(visibleKeys.all().length <= 0) { return []; }
      return [];
      var skpPhrase = 'Update SKP version to ';
      var updates = commitDict.lazySearch(function(commitData) {
        return commitData['commit_msg'] && 
            commitData['commit_msg'].slice(0, skpPhrase) == skpPhrase;
      }).map(function(commitData) {
        return [parseInt(commitData['commit_msg'].substr(skpPhrase.length)),
            commitData['commit_time']];
      });
      var markings = [];
      for(var i = 1; i < updates.length; i++) {
        if(updates[i][0] % 2 == 0) {
          markings.push([updates[i-1][1], updates[i][1]]);
        }
      }
      return markings.map(function(pair) {
        return { xaxis: {from: pair[0], to: pair[1]}, color: '#cccccc'};
      });
    },
    /* Plots the given data.*/
    plot: function(data) {
      // Plot the given data
      plotRef.setData(data);
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
      // Push changes to legend
      legend.updateColors(plotRef);
      legend.refresh();
      // Then push changes to zoom control and date controls
      var range = this.getCurrentRange();
      $$$('#zoom').value = toZoomValue(range[0], range[1]);
      $$$('#start').value = toRFC(range[0]);
      $$$('#end').value = toRFC(range[1]);
    },
    /* Produces a wrapper that can be used in any context.*/
    makePlotCallback: function() {
      var _this = this;
      return function(data) {
        _this.plot(data);
      };
    },
    /* Returns the current visible range. Null to load the latest tile.*/
    getCurrentRange: function() {
      return getCurRange();
    },
    /* Sets the plot to display the key next time it is plotted.*/
    show: function(key) {
      if(!visibleKeys.has(key)) {
        visibleKeys.push(key);
      }
    },
    /* Hides a trace from the plot the next time it is plotted.*/
    hide: function(key) {
      visibleKeys.remove(key);
    },
    debug: function() {
      return [plotRef, visibleKeys];
    }
  };
})();


document.addEventListener('DOMContentLoaded', function() {
  console.log('DOM loaded, init()ing components');
  commitDict.init();
  plotData.init();
  traceDict.init();
  schema.init();
  legend.init(function(key) {
    plotData.show(key);
  }, function(key) {
    plotData.hide(key);
  }, function() {
    plotData.data(plotData.makePlotCallback());
  });

  document.body.addEventListener('click', function(e) {
    if(!$(e.target).parents().is('#note,#chart')) {
      $('#note').hide();
    }
  });


  $('#note').hide();
  $('#notification').hide();
});
