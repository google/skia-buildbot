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
     unt: true 
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
    var ret = sk.query.fromObject(srcState); 
    if (ret === '') {
      return ''; 
    }
    return '?' + ret; 
  }; 

  // setQuery sets the current query on the URL without reloading. 
  gold.setUrlQuery = function(q) {
    history.pushState(null, "", window.location.origin + window.location.pathname + q);
  };
})();
