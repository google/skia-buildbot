'use strict';


/* Add this to the skia namespace */
var skia = skia || {};
(function (ns) {
  // c contains all constants. Primarily relating to backend resources.
  // They need to match the definitions in go/skiacorrectness/main.go
  ns.c = {
    // PREFIX_URL is the prefix to all backend request to JSON resources.
    PREFIX_URL: '/rest',

    // URLs exposed by the backend to retrieve JSON data.
    URL_COUNTS: '/counts',
    URL_TRIAGE: '/triage',
    URL_STATUS: '/status',

    URL_LOGIN_STATUS: '/loginstatus',
    URL_LOGOUT: '/logout',

    // The triage labels need to match the values in golden/go/types/types.go
    UNTRIAGED: 0,
    POSITIVE: 1,
    NEGATIVE: 2,

    // Key in parameters that identifies a test.
    PRIMARY_KEY_FIELD: 'name',

    CORPUS_FIELD: 'source_type',

    // Param fields to filter.
    PARAMS_FILTER: {
      'source_type': true
    },

    // Query parameters (that are not automatically available via parameters).
    QUERY_COMMIT_START: 'cs',
    QUERY_COMMIT_END: 'ce'
  };

  // List of states - used to cycle through via nextState.
  ns.c.ALL_STATES = [ns.c.UNTRIAGED, ns.c.POSITIVE, ns.c.NEGATIVE];

    /**
     * Plot is a class that wraps the flot object and exposes draw functions.
     *
     * @param {jQueryElement} element This a jquery element to which the
                                      flot instance to attach to.
     *
     * @return {Plot} instance of Plot class.
     **/

  ns.Plot = function (element) {
    this.element = element;

    // initialize the flot element with empty data.
    this.flotObj = element.plot([], {
      legend: {
        show: true
      },
      xaxis: {
        show: true
      }
    }).data('plot');
  };


  /**
   * setData sets the data that the plot needs to draw and forces a redraw.
   * If ticks is not null it will also set the ticks and reset the x-axis.
   *
     * @param {array} data Array of series understood by Flot. See
     *           https://github.com/flot/flot/blob/master/API.md#data-format
     *
     * @param {array} ticks Array or function that defines the ticks for the
     *                      x-axis.
  */
  ns.Plot.prototype.setData = function(data, ticks) {
    this.flotObj.setData(data);

    // Set the ticks on the x axis if necessary.
    if (ticks) {
      var opt = this.flotObj.getOptions();
      opt.xaxes.forEach(function(axis) {
        axis.ticks = ticks;
      });
    }

    this.redraw();
  };

  /**
  * redraw forces a resize and redraw of the canvas.
  */
  ns.Plot.prototype.redraw = function () {
    // redraw the graph
    this.flotObj.resize();
    this.flotObj.setupGrid();
    this.flotObj.draw();
  }

  /**
   * PlotData is a class that used as the return value of processAllCounts and
   * contains the processed data.
   *  */
  ns.PlotData = function (data, ticks, allAggregates, testDetails) {
    this.plotData = data;
    this.ticks = ticks;
    this.testDetails = testDetails;
    this.allAggregates = allAggregates;
  };

  /**
   * getTicks returns the ticks for the PlotData object at hand.
   */
  ns.PlotData.prototype.getTicks = function (axis) {
    return this.ticks;
  };

  /**
  * TestDetails is a class that contails the aggregated information about
  * a single tests. It is derived from the data returned by the server.
  */
  ns.TestDetails = function (name, counts) {
    this.name = name;
    this.counts = counts;
    this.aggregates = aggregateCounts(counts);
  };

  /**
  * DiffDigestInfo is a helper class to store information about a
  * digest (usually positive) and how it differs from the a given
  * untriaged digest.
  */
  ns.DiffDigestInfo = function (digest, imgUrl, count, paramCounts, diff) {
    this.digest = digest;
    this.imgUrl = imgUrl;
    this.count = count;
    this.paramCounts = paramCounts;
    this.diff = diff;
  };

  /**
  * isIdentical returns true if the current digest is identical
  *             to the untriaged digest.
  */
  ns.DiffDigestInfo.prototype.isIdentical = function () {
    return this.diff.numDiffPixels === 0;
  }

  /**
  * addIndexAsX adds takes an array of numbers and returns an array of
  * datapoints (x,y) where x is the index of the input element y.
  */
  function addIndexAsX(arr) {
    var result = [];
    for(var i=0, len=arr.length; i<len; i++) {
      result.push([i, arr[i]])
    }
    return result;
  }

  /**
  * aggregateCounts sums over the counts contained in an object.
  * Each member in the object is assumed to be an array of numbers.
  *
  * @param { object } countsObj contains attributes where each attribute
  *                   is an array of numbers.
  * @return {object} an array with the same attributes as the input object.
  *                  Each attribute contains the sum of the corresponding
  *                  array.
  */
  function aggregateCounts(countsObj) {
    var result = {};
    var arr;
    for(var k in countsObj) {
      if (countsObj.hasOwnProperty(k)) {
        result[k] = 0;
        arr = countsObj[k];
        for(var i=0,len=arr.length; i < len; i++) {
          result[k] += arr[i];
        }
      }
    }
    return result;
  }

  /**
  * processAllCounts converts the data returned by the server to
  *  an instance of PlotData that can then be used to render the UI
  *  and also serve as input to the Plot class.
  *
  * @param {object} serverData returned from the server containing the
  *                 aggregated values over all tests.
  *
  * @param {string} testName specifies whether we want to get the data
  *                 for a specific test. If null all data are returned.
  *
  * @return {object} instance of PlotData.
  *
  */
  ns.processCounts = function (serverData, testName) {
    // get the counts from the tests.
    var testCounts = [];
    if (testName && serverData.counts.hasOwnProperty(testName)) {
      testCounts.push(new ns.TestDetails(testName, serverData.counts[testName]))
    }
    else {
      for(var tName in serverData.counts) {
        if (serverData.counts.hasOwnProperty(tName)) {
          testCounts.push(new ns.TestDetails(tName, serverData.counts[tName]));
        }
      }
    }

    // assemble the plot data.
    var targetData = testName ? serverData.counts[testName] : serverData.aggregated;
    var data = [];
    for(var k in targetData) {
      if (targetData.hasOwnProperty(k)) {
        data.push({
          label: k,
          lines: {
            show: true,
            steps: true
          },
          data: addIndexAsX(targetData[k])
        });
      }
    }

    return new ns.PlotData(data,
                           serverData.ticks,
                           aggregateCounts(serverData.aggregated),
                           testCounts);
  };



  /**
    getAutoCommitRanges returns a list of decreasing commit ranges
    that can be used to render a simple selection on the screen.
    TODO: Make this adaptive on the backend so that the minimum is not
    5 but whatever number of commits cover all traces aka constitute
    out current knowledge of HEAD.
  */
  var COMMIT_INTERVALS = [100, 50, 20, 15, 10, 5];
  ns.getAutoCommitRanges = function(serverData) {
    var commits = serverData.commits;
    var result = [{ start: commits[0], name: commits.length }];
    for(var i=0, len=COMMIT_INTERVALS.length; i < len; i++) {
      if (commits.length > COMMIT_INTERVALS[i]) {
        result.push({ start: commits[COMMIT_INTERVALS[i]-1], name: COMMIT_INTERVALS[i] });
      }
    }

    return result;
  };

  /**
  * extractTriageData is the central function to pre-process data coming
  *                   from the server.
  */
  ns.extractTriageData = function (serverData, filterParams) {
    var result = [];
    var triageState = {};
    var totalUnt = new Stats(),
        totalPos = new Stats(),
        totalNeg = new Stats();

    for (var i = 0, len = serverData.tests.length; i < len; i++) {
      var untStats = new Stats();
      var posStats = new Stats();
      var negStats = new Stats();
      var positive = ns.getSortedDigests(serverData, 'positive', posStats, i);
      var negative = ns.getSortedDigests(serverData, 'negative', negStats, i);
      var untriaged = ns.getUntriagedSorted(serverData, i, untStats);
      var testName = serverData.tests[i].name;

      // Set the triage state for each digest.
      var add = function(arr, state) {
        for (var i=0, len=arr.length; i<len; i++) {
          triageState[testName][arr[i].digest] = state;
        }
      };

      triageState[testName] = {};
      add(positive[0], ns.c.POSITIVE);
      add(negative[0], ns.c.NEGATIVE);
      add(untriaged, ns.c.UNTRIAGED);

      result.push({
        name: testName,
        untriaged: untriaged,
        positive:  positive[0],
        positiveMap: positive[1],
        negative:  negative[0],
        negativeMap: negative[1],
        untStats:  untStats,
        posStats:  posStats,
        negStats:  negStats
      });

      totalPos.add(posStats);
      totalNeg.add(negStats);
      totalUnt.add(untStats);
    }

    return {
      tests: result,
      allParams: ns.getSortedParams(serverData, filterParams),
      triageState: triageState,
      commitRanges: ns.getAutoCommitRanges(serverData),
      untStats: totalUnt,
      posStats: totalPos,
      negStats: totalNeg
    };
  };

  /**
  * extractTriageListData returns test data across tests from data returned
  * by the server. Currently shows number of unques digests for each test.
  */
  ns.extractTriageListData = function (serverData) {
    var result = [];
    var tests = serverData.tests;
    for (var i=0, len=tests.length; i < len; i++) {
      var t = tests[i];
      var  posK = ns.keys(t.positive);
      var  negK = ns.keys(t.negative);
      var  untK = ns.keys(t.untriaged);
      var showPos = (posK.length > 0) && t.positive[posK[0]];
      var showUnt = (untK.length > 0) && t.untriaged[untK[0]];
      var showDif = (posK.length > 0) && (untK.length > 0);

      var untStats = new Stats();
      var untriaged = ns.getUntriagedSorted(serverData, i, untStats);

      // Return the stats element and the list of untriaged Digests.
      result.push({
        name: t.name,
        posLen: posK.length,
        negLen: negK.length,
        untLen: untK.length,
        showPos: showPos,
        showUnt: showUnt,
        showDif: showDif,
        untriaged: untriaged
      });
    }

    result.sort(function(a,b) {
      return a.name < b.name ? -1 : a.name > b.name ? 1 : 0;
    });

    return {
      tests: result,
      allParams: ns.getSortedParams(serverData, false),
      commitRanges: ns.getAutoCommitRanges(serverData)
    };
  };

  /**
  * Stats is a helper class to hold counts about a set of digests.
  */
  function Stats(total, unique) {
    this.total = total || 0;
    this.unique = unique || 0;
  }

  Stats.prototype.set = function (total, unique) {
    this.total = total;
    this.unique = unique;
  };

  Stats.prototype.add = function (statsObj) {
    this.total += statsObj.total;
    this.unique += statsObj.unique;
  };

  /**
  *  getUntriagedSorted returns the untriaged digests sorted by largest
  *  deviation from a positively labeled digest. It processes the data
  *  directly returned by the backend.
  *  It also resolves the references to the positive digests contained in
  *  the diff metrics.
  */
  ns.getUntriagedSorted = function(serverData, testIdx, stats) {
    var unt = robust_get(serverData, ['tests', testIdx, 'untriaged']);
    if (!unt) {
      return [];
    }

    var posd, d;
    var result = [];
    var positive = serverData.tests[testIdx].positive;
    var hasPos = false;
    var total = 0;

    for (var digest in unt) {
      if (unt.hasOwnProperty(digest)) {
        total += unt[digest].count;
        var posDiffs = [];
        for(var i=0, len=unt[digest].diffs.length; i < len; i++) {
          // TODO (stephana): Fill in expanding the diff information.
          // This will be done once triaging works. So we can test it
          // with real data.
          hasPos = true;
          d = unt[digest].diffs[i];
          posd = positive[d.posDigest];
          posDiffs.push(new ns.DiffDigestInfo(d.posDigest, posd.imgUrl,
                                           posd.count, posd.paramCounts, d));
        }

        // Inject the digest and the augmented positive diffs.
        unt[digest].digest = digest;
        unt[digest].positiveDiffs = posDiffs;
        unt[digest].paramCounts = ns.filterObject(unt[digest].paramCounts, ns.c.PARAMS_FILTER);
        result.push(unt[digest]);
      }
    }

    stats.set(total, result.length);

    // Sort the result increasing by pixel difference or
    // decreasing by counts if there are no positives.
    var sortFn;
    if (hasPos) {
      sortFn = function (a,b) {
        return a.positiveDiffs[0].diff.numDiffPixels - b.positiveDiffs[0].diff.numDiffPixels;
      };
    } else {
      sortFn = function (a,b) { return b.count - a.count; };
    }
    result.sort(sortFn);

    return result;
  };

  /**
  * getSortedPositivesFromUntriaged returns the list of positively labeled
  * digests. It assumes that 'untriagedRec' was generated by a call to
  * getUntriagedSorted(...).
  */
  ns.getSortedPositivesFromUntriaged = function (untriagedRec) {
    if (untriagedRec && untriagedRec.positiveDiffs && untriagedRec.positiveDiffs.length > 0) {
      return untriagedRec.positiveDiffs;
    }

    return [];
  };

  /**
  * getSortedDigests returns a list of digests with the given digestClass
  * from the data returnded by the backend. This is to be used when there are no
  * untriaged digests.
  */
  ns.getSortedDigests= function (serverData, digestClass, stats, idx) {
    var targetDigests = robust_get(serverData, ['tests', idx, digestClass]);
    if (!targetDigests)  {
      return [[], {}];
    }

    var result = [];
    var total = 0;
    for (var digest in targetDigests) {
      if (targetDigests.hasOwnProperty(digest)) {
        total += targetDigests[digest].count;
        // Inject the digest into the object.
        targetDigests[digest].digest = digest;
        targetDigests[digest].paramCounts = ns.filterObject(
                        targetDigests[digest].paramCounts, ns.c.PARAMS_FILTER);
        result.push(targetDigests[digest]);
      }
    }

    stats.set(total, result.length);

    // Sort the result in decreasing order of their occurences.
    result.sort(function (a,b) {
      a.count - b.count;
    });

    return [result, targetDigests];
  };

  /**
  * getSortedParams returns all parameters and the union of their values as a
  * (nested) sorted Array in the format:
  *       [[param1, [val1, val2, ...],
           [param2, [val3, val4, ...], ... ]]]
  */
  ns.getSortedParams = function (serverData, filter) {
    var result = [];
    for(var k in serverData.allParams) {
      if (serverData.allParams.hasOwnProperty(k) && (!filter || !ns.c.PARAMS_FILTER[k])) {
          serverData.allParams[k].sort();
          result.push([k, serverData.allParams[k]]);
      }
    }

    result.sort(function(a,b){
      return (a[0] < b[0]) ? -1 : (a[0] > b[0]) ? 1 : 0;
    });

    return result;
  };

  /**
  * filterQueryByParams returns a suboject of query only containing
  * the members that are in sortedParams. sortedParams is assumed to be
  * the same format as returned by the getSortedParams function.
  */

  ns.filterQueryByParams = function (query, sortedParams) {
    if  (!sortedParams || !query) {
      return {};
    }

    var result = {};
    for (var i=0, len=sortedParams.length; i<len; i++) {
      if (query.hasOwnProperty(sortedParams[i][0])) {
        result[sortedParams[i][0]] = query[sortedParams[i][0]];
      }
    }
    return result;
  }

  ns.splitQuery = function (query, allParams) {
    var paramQuery = ns.filterQueryByParams(query, allParams);
    var crq = {};
    crq[ns.c.QUERY_COMMIT_START] = robust_get(query, [ns.c.QUERY_COMMIT_START, 0]) || "";
    crq[ns.c.QUERY_COMMIT_END] = robust_get(query, [ns.c.QUERY_COMMIT_END, 0]) || "";

    return {
      paramQuery: paramQuery,
      commitRangeQuery: crq
    };
  }

  // sortedKeys returns the keys of the object in sorted order.
  ns.sortedKeys = function(obj) {
    var result = ns.keys(obj);
    result.sort();
    return result;
  };

  // keys returns the keys of an object.
  ns.keys = function(obj) {
    if (obj.keys) {
      return keys();
    }
    var result = [];
    for(var k in obj) {
      if (obj.hasOwnProperty(k)) {
        result.push(k);
      }
    }
    return result;
  };

  // Returns an object as an array sorted by keys. This assumes that the keys
  // are strings.
  ns.getSortedObject = function(obj) {
    var result = [];
    for(var k in obj) {
      if (obj.hasOwnProperty(k)) {
        result.push([k, obj[k]]);
      }
    }

    result.sort(function(a,b) {
      return a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0;
    });

    return result;
  };

  /**
  * getCombinedParamsTable takes a variable number of paramCounts and
  * combines them into a multi dimensional array to be displayed as a
  * table. The output format is:
  *    [
  *     { p: 'paramName1', c: [['val1','val2'],['val3', 'val4']] },
  *     { p: 'paramName2', c: [['valx','valy'],['val3', 'val4']] },
  *     { p: 'paramName3', c: [['val1','val2'],['val3', 'val4']] },
  *     { p: 'paramName4', c: [['val1','val2'],['val3', 'val4']] }
  *    ]
  * The array is sorted by values of 'p'.
  */
  ns.getCombinedParamsTable = function ( _ ) {
    var combined = {};
    for(var i=0, len=arguments.length; (i<len) && arguments[i]; i++) {
      var params = arguments[i];
      for(var k in params) {
        if (params.hasOwnProperty(k)) {
          if (!combined[k]) {
            combined[k] = [];
          }
          combined[k][i] = ns.sortedKeys(params[k]);
        }
      }
    }

    var result = [];
    for(var k in combined) {
      if (combined.hasOwnProperty(k)) {
        result.push({p: k, c: combined[k]});
      }
    }

    result.sort(function(a,b) {
      return (a.p < b.p) ? -1 : (a.p > b.p) ? 1 : 0;
    });

    return result;
  };

  /**
  * nextState returns the next state in the order defined by ALL_STATES.
  */
  ns.nextState = function(state) {
    var idx = (ns.c.ALL_STATES.indexOf(state) + 1) % ns.c.ALL_STATES.length;
    return ns.c.ALL_STATES[idx];
  };

  /**
  * getNumArray returns an array of numbers of the given length and each
  * element is initialized with initVal. If initVal is omitted, 0 (zero)
  * is used instead.
  */
  ns.getNumArray = function(len, initVal) {
    if (!initVal) {
      initVal = 0;
    }

    var result = [];
    for(var i =0; i < len; i++) {
      result.push(initVal);
    }

    return result;
  };

  /**
  * updateDelta updates the given delta between changed and original for the
  * given testname. If testName evaluates to false the deltas for all tests
  * are calculated. It returns the number of changes this delta would result
  * in.
  */
  ns.updateDelta = function (changed, original, delta, testName) {
    var useTestNames = (testName) ? [testName] : ns.keys(changed);
    var testDelta, testCount, tn;

    for (var i = 0, len = useTestNames.length; i < len; i++) {
      tn = useTestNames[i];
      if (changed.hasOwnProperty(tn)) {
        testDelta = {};
        testCount = 0;

        for (var k in changed[tn]) {
          if (changed[tn].hasOwnProperty(k) &&
              original[tn].hasOwnProperty(k) &&
              (changed[tn][k] !== original[tn][k])) {
            testDelta[k] = changed[tn][k];
            testCount++;
          }
        }

        delta[tn] = {
          delta: testDelta,
          count: testCount
        };
      }
    }

    var pendingCount = 0;
    for(var k in delta) {
      if (delta.hasOwnProperty(k)) {
        pendingCount += delta[k].count;
      }
    }

    return pendingCount;
  };

  /**
  * filterObject returns a copy of 'obj' without the members that
  *              are also members in 'exclude'.
  */
  ns.filterObject = function(obj, exclude) {
    var result = {};
    for(var k in obj) {
      if (obj.hasOwnProperty(k) && !exclude[k]) {
        result[k] = obj[k];
      }
    }

    return result;
  };

  /**
  * subObject returns a new object with only the members that are
  * also in include which is expected to be an array of strings.
  */
  ns.subObject = function(obj, include) {
    var result = {};
    for (var i=0, len=include.length; i<len; i++) {
      if (obj.hasOwnProperty(include[i])) {
        result[include[i]] = obj[include[i]];
      }
    }
    return result;
  };

  /**
  * unionObject returns a new object with the union of all the members
  * of the arbibrary number of objects that are supplied.
  */
  ns.unionObject = function( _ ) {
    var result = {};
    for(var i=0, len=arguments.length; i<len; i++) {
      for(var k in arguments[i]) {
        if (arguments[i].hasOwnProperty(k)) {
          result[k] = arguments[i][k];
        }
      }
    }
    return result;
  }

  /**
  * TriageDigestReq is a container type for sending labeled digests to the
  * backend. It matches the input parameters of the triageDigestsHandler in
  * 'go/skiacorrectness/main.go'.
  */

  ns.TriageDigestReq = function () {
  };

  /**
  * addDigestLabel is a convenience method to add digests and their label to the
  * the instance.
  */
  ns.TriageDigestReq.prototype.add = function (testName, digest, label) {
    this[testName] = this[testName] || {};
    this[testName][digest] = label;
  };

  /////////////////////////////////////////////////////////////////
  // Generic utility functions.
  /////////////////////////////////////////////////////////////////
  /*
  * isEmpty returns true if the provided object is empty and false
  *         otherwise.
  */
  ns.isEmpty = function (obj) {
    for (var k in obj) {
      if (obj.hasOwnProperty(k)) {
        return false;
      }
    }
    return true;
  };

  /*
  */
  ns.extractQueryString = function (url) {
    var idx = url.indexOf('?');
    return (idx === -1) ? '' : url.substring(idx);
  };

  /////////////////////////////////////////////////////////////////
  // Utility functions that are not exposed in the namespace.
  /////////////////////////////////////////////////////////////////

  /**
   * robust_get finds a sub object within 'obj' by following the path
   * in 'idx'. It will not throw an error if any sub object is missing
   * but instead return 'undefined'.
   **/
  function robust_get(obj, idx) {
    if (!idx) {
      return;
    }

    for(var i=0, len=idx.length; i<len; i++) {
      if ((typeof obj === 'undefined') || (typeof idx[i] === 'undefined')) {
        return;  // returns 'undefined'
      }

      obj = obj[idx[i]];
    }

    return obj;
  }


})(skia);
