/**
 * Navigation coordinates the <plot-sk>, <query-sk>, and <trace-details-sk>
 * elements on that main page of skiaperf.
 *
 */
(function() {
  "use strict";

  /**
   * Notifies the user.
   *
   * TODO(jcgregorio) Move to toast.
   */
  function notifyUser(err) {
    alert(err);
  }


  /**
   * Navigation coordinates the <plot-sk>, <query-sk>, and <trace-details-sk>
   * elements on that main page of skiaperf.
   */
  function Navigation() {
    // Keep tracking if we are still loading the page the first time.
    this.loading_ = true;

    this.commitData_ = [];
  };

  /**
   * commitData_ may have a trailing set of commits with a commit_time of 0,
   * which means there's no commit, it is just extra space from the Tile.
   */
  Navigation.prototype.lastCommitIndex = function() {
    for (var i = this.commitData_.length - 1; i >= 0; i--) {
      if (this.commitData_[i].commit_time != 0) {
        return i;
      }
    }
    // We shouldn't get here.
    return this.commitData_.length-1;
  }

  /**
   * Adds Traces that match the given query params.
   *
   * q is a URL query to be appended to /query/<scale>/<tiles>/traces/.
   * The matching traces are returned and added to the plot.
   */
  Navigation.prototype.addTraces = function(q) {
    var that = this;
    sk.get("/query/0/-1/traces/?" + q).then(JSON.parse).then(function(json){
      $$$('plot-sk').addTraces(json.traces);
      if (json["hash"]) {
        var index = -1;
        for (var i = 0, len = that.commitData_.length; i < len; i++) {
          if (that.commitData_[i].hash == json["hash"]) {
            index = i;
            break;
          }
        }
        $$$('plot-sk').stepIndex = index;
      }
    }).then(function(){
      that.loading_ = false;
    }).catch(notifyUser);
  };

  Navigation.prototype.addCalculatedTrace = function(formula) {
    var navigation = this;
    sk.get("/calc/?formula=" + encodeURIComponent(formula)).then(JSON.parse).then(function(json){
      $$$('plot-sk').addTraces(json.traces);
    }).then(function(){
      navigation.loading_ = false;
    }).catch(notifyUser);
  };

  Navigation.prototype.clearShortcut = function() {
    if (this.loading_ == false) {
      window.history.pushState(null, "", "#");
    }
  }

  /**
   * Wires up all the callbacks to the controls that Navigation uses.
   */
  Navigation.prototype.attach = function() {
    var that = this;

    $$$('#add-lines').addEventListener('click', function() {
      that.clearShortcut();
      that.addTraces($$$('query-sk').currentQuery);
    });

    $$$('#add-calculated').addEventListener('click', function() {
      that.clearShortcut();
      that.addCalculatedTrace($$$('#formula').value);
    });

    // Update the formula when the query changes.
    $$$('query-sk').addEventListener('change', function(e) {
      var formula = $$$('#formula').value;
      var query = e.detail;
      if (formula == "") {
        $$$('#formula').value = 'filter("' + query + '")';
      } else if (2 == (formula.match(/\"/g) || []).length) {
        // Only update the filter query if there's one string in the formula.
        $$$('#formula').value = formula.replace(/".*"/, '"' + query + '"');
      }
    });

    $$$('#shortcut').addEventListener('click', function() {
      // Package up the current state and stuff it into the database.
      var state = {
        scale: 0,
        tiles: [-1],
        keys: $$$('plot-sk').getKeys()
      };
      if (state.keys.length > 0) {
        sk.post("/shortcuts/", JSON.stringify(state)).then(JSON.parse).then(function(json) {
          // Set the shortcut in the hash.
          window.history.pushState(null, "", "#" + json.id);
        });
      } else {
        notifyUser("Nothing to shortcut.");
      }
    });

    $$$('#nuke-plot').addEventListener('click', function(e) {
      $$$('plot-sk').clear();
    });

    $$$('plot-sk').addEventListener('selected', function(e) {
      // Convert the commit index to actual git hash.
      var beginHash = that.commitData_[e.detail.begin].hash;
      var endHash = undefined;
      if (e.detail.end) {
        endHash = that.commitData_[e.detail.end].hash;
      }
      $$$('trace-details-sk').displayRange(beginHash, endHash);
      $$$('trace-details-sk').setParams(e.detail.id, e.detail.params);
    });

    $$$('plot-sk').addEventListener('highlighted', function(e) {
      $$$('highlightbar-sk').key = e.detail.id;
      $$$('highlightbar-sk').value = e.detail.value.toPrecision(5);
    });

    $$$('trace-details-sk').addEventListener('highlightGroup', function(e) {
      $$$('plot-sk').highlightGroup(e.detail.key, e.detail.value);
    });

    $$$('trace-details-sk').addEventListener('only', function(e) {
      that.clearShortcut();
      $$$('plot-sk').only(e.detail.id);
    });

    $$$('trace-details-sk').addEventListener('group', function() {
      that.clearShortcut();
      $$$('plot-sk').removeUnHighlighted();
    });

    $$$('trace-details-sk').addEventListener('remove', function(e) {
      that.clearShortcut();
      $$$('plot-sk').remove(e.detail.id);
    });

    $$$('#reset-axes').addEventListener('click', function(e) {
      $$$('plot-sk').resetAxes();
    });

    // Load the commit data and set up the plot.
    sk.get('/tiles/0/-1/').then(JSON.parse).then(function(json){
      that.commitData_ = json.commits;
      $$$('query-sk').setParamSet(json.paramset);
      if (window.location.hash.length >= 2) {
        that.addTraces("__shortcut=" + window.location.hash.substr(1))
      }

      var skps = [0].concat(json.skps, [that.commitData_.length-1]);
      $$$('plot-sk').setBackgroundInfo(json.ticks, skps, that.lastCommitIndex());
    });

  };

  sk.domReady().then(function() {
    var navigation = new Navigation();
    navigation.attach();
  });

}());
