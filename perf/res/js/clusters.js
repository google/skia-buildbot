// Copyright (c) 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.
//

(function() {
  // Copied from perf/server/res/js/logic.js
  // TODO(bensong): move to a common file
  var id = function(e) { return e; };

  function $$(query, par) {
    if(!par) {
      return Array.prototype.map.call(document.querySelectorAll(query), id);
    } else {
      return Array.prototype.map.call(par.querySelectorAll(query), id);
    }
  };

  // doSort sorts the clustering results with the algorithm given in element e.
  function doSort(e) {
    if (!e.target.value) {
      return;
    }
    var container = document.getElementById("container");
    var to_sort = [];
    $$('div', container).forEach(function(ele) {
      var data = ele.dataset;
      if (!data.clustersize || !data.stepdeviation || !data.stepsize) {
        return;
      }
      to_sort.push([+data.clustersize, +data.stepdeviation, +data.stepsize, ele]);
    });
    switch(e.target.value) {
      case "stepFit":
        to_sort.sort(function(x, y) {
          return x[1]/x[2] - y[1]/y[2];
        });
        break;
      case "stepSize":
        to_sort.sort(function(x, y) {
          return y[2] - x[2];
        });
        break;
      case "stepDeviation":
        to_sort.sort(function(x, y) {
          return x[1] - y[1];
        });
        break;
      case "clusterSize":
      default:
        to_sort.sort(function(x, y) {
          return y[0] - x[0];
        });
    }
    to_sort.forEach(function(i) {
      container.appendChild(i[3]);
    });
  };

  function toggle(e) {
    e.target.nextElementSibling.classList.toggle("display");
  };

  // hookExpando finds all the expander buttons and adds a handler
  // that toggles the 'display' class on its next sibling element.
  // TODO(jcgregorio) Switch to details/summary once we have a polyfill in place.
  function hookExpando() {
    $$('.expander').forEach(function(ele) {
      ele.addEventListener('click', toggle);
    });
  };

  // hookSort finds all the radio buttons and adds a handler that sorts the
  // clustering results with various algorithms.
  function hookSort() {
    $$('input[name="sort"]').forEach(function(ele) {
      ele.addEventListener('click', doSort);
    });
  };

  function hookClicks() {
    hookExpando();
    hookSort();
  };

  document.addEventListener('DOMContentLoaded', hookClicks);

})();
