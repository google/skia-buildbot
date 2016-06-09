"use strict"

/* This defines the gold namespace which contains all JS code relevant to
   gold. Functions that are generic should move to common.js 

   TODO(stephana): Move everythin the requires to be in sync with the 
   backend to this file. 
   */

var gold = gold || {};

(function(){

  // Default values for the search controls. 
  gold.defaultSearchState = {
     query:   "source_type=gm",
     head:    true,
     include: false,
     pos: false,
     neg: false, 
     unt: true,
     blame: "",
     issue: "",
     patchsets: ""
  };

  // Table that maps reference point ids to readable titles. 
  gold.diffTitles = {
    "tpos" : "Trace positive",
    "pos" : "Closest Positive",
    "neg" : "Closest Negative"
  }; 

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
  gold.diffQuery = function(test, left, right) {
    return '?test=' + test + '&left=' + left + '&right=' + right;
  };

  // Returns the query string to usee for the detail page or the call to the 
  // diff endpoint. 
  gold.detailQuery = function(test, digest) {
    return '?test=' + test + '&digest=' + digest;
  };

  // stateFromQuery returns a state object based on the query portion of the URL. 
  gold.stateFromQuery = function(defaultState) {
    var delta = sk.query.toObject(window.location.search.slice(1), defaultState);
    return sk.object.applyDelta(delta, defaultState);
  }; 

  // queryFromState returns a query string from the the given state object.
  gold.queryFromState = function(srcState) {
    // Filter out empty strings. 
    var cpState = {};
    for(var k in srcState) {
      if (srcState.hasOwnProperty(k) && (srcState[k] !== '')) {
        cpState[k] = srcState[k]; 
      }
    }

    var ret = sk.query.fromObject(cpState); 
    if (ret === '') {
      return ''; 
    }
    return '?' + ret; 
  }; 

  // loadWithActivity sends a GET request to the given url and uses the provide 
  // acitivity element as an indicator. If the call succeeds it applies the 
  // parsed result to 'target'. 
  // If 'target' is a string it will call the 'set' function of the Polymer 
  // element 'ele' with 'target', if 'target' is a function it will call it. 
  gold.loadWithActivity = function(ele, url, activity, target) {
    activity.startSpinner("Loading...");
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

    // _initState initializes the "_state" and "_ctx" variables. ctx is the 
    // context of the page.js route. It creates the value of the _state object 
    // from the URL query string based on defaultState. It sets the URL to 
    // the resulting the state. 
    _initState: function(ctx, defaultState) {
      this._ctx = ctx; 
      this._state = gold.stateFromQuery(defaultState); 
      this._setUrlFromState();
    },

    // _redirectToState updates the current state with 'updates'. After it 
    // saves the current URL to history it redirects (via history.replaceState) 
    // to the same path page with a query string that represents the 
    // updated state. 
    _redirectToState: function(updates) {
      // Save the current history entry before the redirect.
      this._ctx.pushState();
      var newState = sk.object.applyDelta(updates, this._state);
      page.redirect(window.location.pathname + gold.queryFromState(newState));
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
    }
  };

  // ZoomTargetBehavior adds handling of zoom-clicked events to an element. 
  // It requires that a <zoom-dialog-sk> element is in the markup of the 
  // element that uses this behavior. 
  gold.ZoomTargetBehavior = {
    ready: function() {
      this.listen(this, 'zoom-clicked', '_handleZoomClicked');
    }, 

    _handleZoomClicked: function(ev) {
      $$$('zoom-dialog-sk', this).open(ev.detail); 
    }
  }; 

})();
