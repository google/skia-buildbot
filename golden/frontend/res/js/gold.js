"use strict";

/* This defines the gold namespace which contains all JS code relevant to
   gold. Functions that are generic should move to common.js

   TODO(stephana): Move everythin the requires to be in sync with the
   backend to this file.
   */

var gold = gold || {};

(function(){
  // Constants for status values.
  gold.POSITIVE = 'positive',
  gold.NEGATIVE = 'negative';
  gold.UNTRIAGED = 'untriage';

  // Reference diffs.
  gold.REF_NEG   = 'neg';
  gold.REF_POS   = 'pos';
  gold.REF_TRACE = 'trace';

  // Metric values.
	gold.METRIC_COMBINED = 'combined';
	gold.METRIC_PERCENT  = 'percent';
	gold.METRIC_PIXEL    = 'pixel';
  gold.allMetrics = [
    gold.METRIC_COMBINED,
    gold.METRIC_PERCENT,
    gold.METRIC_PIXEL,
  ];

  // Operators to apply to images grouped by test.
  gold.GROUP_TEST_MAX_COUNT = 'count'    // Most often occuring digest.
  gold.groupTestOps = [
    gold.GROUP_TEST_MAX_COUNT,
  ];

  // ISSUE_TRACKER_URL is the url of the monorail issue tracker.
  var ISSUE_TRACKER_URL = 'https://bugs.chromium.org/p/skia/issues/';

  // Costants for sort order.
  gold.SORT_ASC = 'asc';
  gold.SORT_DESC = 'desc';
  gold.sortOptions = [
    gold.SORT_ASC,
    gold.SORT_DESC
  ];

  // Default values for the search controls.
  gold.defaultSearchState = {
    // The metric to use.
    metric: gold.METRIC_COMBINED,

    // Sort order.
    sort: gold.SORT_DESC,

    // Configs that need to match during comparisons.
    match: ['name'],

    // Note: query is a URL encoded query over the test parameters
    // The fields of query are not fixed but change over time. This requires
    // to encode/decode a query in a separate step when encoding/decoding
    // this entire object.
    query:'',
    rquery: '',
    head: true,
    include: false,
    pos: false,
    neg: false,
    unt: true,
    blame: '',
    limit: 50,
    offset: 0,
    crs: '',
    issue: '',
    patchsets: '',

    // Filter options.
    // Begin and end commits. Must be valid commits.
    fbegin: '',
    fend: '',

    // Select max RGBA difference.
    frgbamin: 0,

    // Select max RGBA difference.
    frgbamax: 255,

    // Select max difference.
    fdiffmax: -1,

    // Group by test and select a specific digest.
    fgrouptest: '',

    // Only include images that have a reference.
    fref: false,

    // master indicates whether to include digests that are also in master
    // when querying tryjob results.
    master: false,
  };

  // Default values for the search query of the by-blame-page.
  gold.defaultByBlameState = {
    query: '',
  };

  // Default values for pagination objects.
  gold.defaultPagination = {
    size: 50,
    offset: 0,
    total: 0
  };

  // Default values for pagination URL state.
  gold.defaultPaginationState = {
    size: gold.defaultPagination.size,
    offset: gold.defaultPagination.offset
  };

  // Table that maps reference point ids to readable titles.
  gold.diffTitles = {}
  gold.diffTitles[gold.REF_TRACE] = 'Trace previously';
  gold.diffTitles[gold.REF_POS] = 'Closest Positive';
  gold.diffTitles[gold.REF_NEG] = 'Closest Negative';

  // Return a title for the given reference point id.
  gold.getDiffTitle = function(diffType) {
    return gold.diffTitles[diffType] || diffType;
  };

  // Return the URL for the given digest.
  gold.imgHref = function(digest) {
    if (!digest) {
      return '';
    }

    return '/img/images/' + digest + '.png'
  };

  // Return the URL for the diff image between the two given digests.
  gold.diffImgHref = function(d1, d2) {
    if (!d1 || !d2) {
      return '';
    }

    return '/img/diffs/' + ((d1 < d2) ? (d1 + '-' + d2) : (d2 + '-' + d1)) + '.png'
  };

  // Returns the query string to pass to the diff page or to the diff endpoint.
  // Input is the name of the test and the two digests to compare.
  gold.diffQuery = function(test, left, right, issue) {
    const u = '?test=' + test + '&left=' + left + '&right=' + right;
    if (issue) {
      return u + '&issue=' +issue;
    }
    return u;
  };

  // Returns the query string to use for the detail page or the call to the
  // diff endpoint.
  gold.detailQuery = function(test, digest, issue) {
    const u = '?test=' + test + '&digest=' + digest;
    if (issue) {
      return u + '&issue=' +issue;
    }
    return u;
  };

  // stateFromQuery returns a state object based on the query portion of the URL.
  gold.stateFromQuery = function(defaultState) {
    var delta = sk.query.toObject(window.location.search.slice(1), defaultState);
    return sk.object.applyDelta(delta, defaultState);
  };

  // filterEmpty returns a copy of the object without fields where the value
  // is an empty string.
  gold.filterEmpty = function(obj) {
    var cpObj = {};
    for(var k in obj) {
      if (obj.hasOwnProperty(k) && (obj[k] !== '')) {
        cpObj[k] = obj[k];
      }
    }
    return cpObj;
  };

  // queryFromState returns a query string from the the given state object.
  gold.queryFromState = function(srcState) {
    var ret = sk.query.fromObject(gold.filterEmpty(srcState));
    if (ret === '') {
      return '';
    }
    return '?' + ret;
  };

  // updateParamsConditionally updates the given paramset 'params' with the
  // paramset 'updateParamSet' if the item is not present in the first.
  // If the 'force' flag it true it will always do the update.
  gold.updateParamsConditionally = function(params, updateParamSet, force) {
    for(var k in updateParamSet) {
      if (updateParamSet.hasOwnProperty(k) && (force || !params.hasOwnProperty(k))) {
        params[k] = updateParamSet[k];
      }
    }
    return sk.query.fromParamSet(params);
  }

  // loadWithActivity sends a GET request to the given url and uses the provide
  // acitivity element as an indicator. If the call succeeds it applies the
  // parsed result to 'target'.
  // If 'target' is a string it will call the 'set' function of the Polymer
  // element 'ele' with 'target', if 'target' is a function it will call it.
  gold.loadWithActivity = function(ele, url, activity, target) {
    activity.startSpinner('Loading...');
    sk.get(url).then(JSON.parse).then(function (json) {
      activity.stopSpinner();
      if (typeof(target) === 'function') {
        target(json);
      } else {
        ele.set(target, json);
      }
    }).catch(function(e) {
      activity.stopSpinner();
      sk.errorMessage(e);
    });
  },

  gold.issueURL = function(issueID) {
    return ISSUE_TRACKER_URL + '/detail?id=' + issueID;
  };

  // TriageQuery returns an object that can be sent as a query to the
  // backend to triage digests. The arguments can either be a 4-tuple:
  //    makeTriageQuery(testName, digests, status, issue)
  // or a pair:
  //    makeTriageQuery(arr, issue)
  // where 'arr' is an array of triples: <testName, digests, status>.
  // Note: 'digests' can either be a single string or an array.
  // 'issue' is the id of the code review issue for which we want to triage.
  // It has to be a positive integer (> 0) to be considered.
  gold.TriageQuery = function(triageList, issue) {
    if (arguments.length > 2) {
      triageList = [[arguments[0], arguments[1], arguments[2]]];
      issue = arguments[3];
    }

    var ret = {};
    triageList.forEach(function(t) {
      var test=t[0], digests=t[1], status=t[2];
      if (!Array.isArray(digests)) {
        digests = [digests];
      }

      var found = ret[test];
      if (!found) {
        ret[test] = {};
        found = ret[test];
      }

      for(var i=0; i < digests.length; i++) {
        found[digests[i]] = status;
      }
    });

    this.testDigestStatus = ret;
    this.setIssue(issue);
  };

  // setIssue is a setter for the issue of a TriageQuery
  // if "" or "0", will be assigned to the master branch.
  gold.TriageQuery.prototype.setIssue = function(issue) {
    this.issue = issue;
  };

  // flattenTriageQuery is the inverse operation of makeTriageQuery.
  // It returns an array of triples where each triple contains:
  //    [testName, digests, status]
  // testName and status are strings and digests is an array of strings.
  gold.flattenTriageQuery = function(q) {
    var ret = [];
    q = q.testDigestStatus
    for(var k in q) {
      if (q.hasOwnProperty(k)) {
        var statusMap = {};
        // iterat over the digests and group by status.
        for(var j in q[k]) {
          if (q[k].hasOwnProperty(j)) {
            var status = q[k][j];
            if (!statusMap[status]) {
              statusMap[status] = [];
            }
            statusMap[status].push(j);
          }
        }
        for(j in statusMap) {
          ret.push([k, statusMap[j], j]);
        }
      }
    }
    return ret;
  };

  // PageStateBehavior is a re-usable behavior what adds the _state and
  // _ctx (page.js context) variables to a Polymer element. All methods are
  // implemented as private since they should only be used within a
  // Polymer element.
  gold.PageStateBehavior = {
    properties: {
      _state: {
        type: Object,
        value: function() { return {}; }
      }
    },

    ready: function() {
      // Find the status element and listen to corpus changes.
      this.async(function() {
        this._statusElement = $$$('gold-status-sk');
        if (this._statusElement) {
          this.listen(this._statusElement, 'corpus-change', '_handleCorpusChange');
        }
      });
    },

    _handleCorpusChange: function(ev) {
      // Only change anything related to corpus if this element is the
      // part of the currently viewed page.
      if (Polymer.dom(this).parentNode.hasAttribute('activepage') && this._hasQuery) {
        if (this._corpusHome) {
          this._redirectHome();
          return;
        }
        var params = sk.query.toParamSet(this._state.query);
        params.source_type = [ev.detail];
        this._redirectToState({query: sk.query.fromParamSet(params)});
      }
    },

    // _setDefaultState sets the default state (usually reflected in the URL)
    // of the is document. 'corpusHome' is a boolean flag that indicates whether
    // a corpus change should redirect to the home page.
    _setDefaultState: function(defaultState, corpusHome) {
      this._defaultState = defaultState;
      this._corpusHome = corpusHome;
      this._hasQuery = defaultState.hasOwnProperty('query');
    },

    // _getDefaultStateWithCorpus returns the default search state of this
    // element (previously set via _setDefaultState) with the current corpus
    // injected.
    _getDefaultStateWithCorpus: function(state) {
        var ret = state || this._defaultState || {};
        if (this._statusElement && this._hasQuery) {
          ret = sk.object.shallowCopy(ret);
          ret.query = sk.query.fromParamSet({source_type: [this._statusElement.corpus]});
        }
        return ret;
    },

    // _initState initializes the '_state' and '_ctx' variables. ctx is the
    // context of the page.js route. It creates the value of the _state object
    // from the URL query string based on defaultState. It sets the URL to
    // the resulting the state.
    _initState: function(ctx, defaultState) {
      this._ctx = ctx;
      this._state = gold.stateFromQuery(defaultState);
      if (this._hasQuery) {
        this._syncCorpusQuery(defaultState.query);
      }
      this._setUrlFromState();
    },

    // _syncCorpusQuery synchronizes the the corpus value between the current
    // request (represented by this._state.query) with the corpus in status.
    // Effectively changing the corpus in status.
    _syncCorpusQuery: function(defaultQueryStr) {
      var defaultParams = sk.query.toParamSet(defaultQueryStr);
      var params = sk.query.toParamSet(this._state.query);
      this._state.query = gold.updateParamsConditionally(params, defaultParams, false);
      if (this._statusElement) {
        this._statusElement.setCorpus(params.source_type[0]);
      }
    },

    // _redirectToState updates the current state with 'updates'. After it
    // saves the current URL to history it redirects (via history.replaceState)
    // to newTargetPath, if provided, otherwise it will use the current path.
    _redirectToState: function(updates, newTargetPath) {
      // Save the current history entry before the redirect.
      this._ctx.pushState();
      page.redirect(this._getRedirectPath(updates, newTargetPath));
    },

    // Calculates a new path given the state update and an optional new target
    // path.
    _getRedirectPath: function(updates, newTargetPath) {
      var newState = sk.object.applyDelta(updates, this._state);
      var targetPath = newTargetPath ||  window.location.pathname;

      return targetPath + gold.queryFromState(newState);
    },

    // _getRedirectURL returns a new URL based on the current state and
    // target path.
    _getRedirectURL: function(updates, newTargetPath) {
      var path = this._getRedirectPath(updates, newTargetPath);
      var host = window.location.protocol + '//' + window.location.hostname;
      return host + ':' + window.location.port + path;
    },

    // _redirectHome unconditionally redirects to home.
    _redirectHome: function() {
      this._ctx.pushState();
      page.redirect('/' + gold.queryFromState(this._getDefaultStateWithCorpus()));
    },

    // _replaceState updates the current state with 'updates' and updates
    // the URL accordingly. No new page is loaded or reloaded.
    _replaceState: function(updates) {
      this._state = sk.object.applyDelta(updates, this._state);
      this._setUrlFromState();
    },

    // setUrlFromState simply replaces the query string of the current URL
    // with a query string that represents the current state.
    _setUrlFromState: function() {
      history.replaceState(this._ctx.state, this._ctx.title, window.location.pathname + gold.queryFromState(this._state));
    },
  };

})();
