"use strict"

/* This defines the gold namespace which contains all JS code relevant to
   gold. Functions that are generic should move to common.js 

   TODO(stephana): Move everythin the requires to be in sync with the 
   backend to this file. 
   */

var gold = gold || {};

(function(){

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

})();
