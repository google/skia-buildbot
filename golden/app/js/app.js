'use strict';

/**
* This angular.js module pulls together the UI and logic.js.
* It only contains code to make requests to the backend and to implement
* UI logic. All application logic is in logic.js.
*/

// Add everything to the skia namespace.
var skia = skia || {};
(function (ns) {

  var app = angular.module('rbtApp', ['ngRoute']);

  // Configure the different within app views.
  app.config(['$routeProvider', function($routeProvider) {
    $routeProvider.when(ns.c.URL_COUNTS + '/:id?', {
          templateUrl: 'partials/counts-view.html',
          controller: 'CountsCtrl',
          reloadOnSearch: false
        });

    $routeProvider.when(ns.c.URL_TRIAGE, {
          templateUrl: 'partials/triage-view.html',
          controller: 'TriageCtrl',
          reloadOnSearch: false
        });

    $routeProvider.when(ns.c.URL_TRIAGE + '/:id', {
          templateUrl: 'partials/triage-details-view.html',
          controller: 'TriageDetailsCtrl',
          reloadOnSearch: false
        });
    $routeProvider.otherwise({redirectTo: ns.c.URL_TRIAGE });
  }]);

  /*
   * CountsCtrl controlls the UI on the main view where an overview of
   * test results is presented.
   */
  app.controller('CountsCtrl', ['$scope', '$routeParams', '$location', '$timeout', 'dataService',
    function($scope, $routeParams, $location, $timeout, dataService) {
      // Get the path and use it for the backend request
      var testName = ($routeParams.id && ($routeParams.id !== '')) ?
                      $routeParams.id : null;

      // Load counts for all tests of a tile.
      $scope.loadCounts = function () {
        $scope.state = 'loading';
        dataService.loadData(ns.c.URL_COUNTS, $scope.query).then(
          function (serverData) {
            if (!serverData) {
              retry();
              return serverData;
            };

            var temp = ns.processCounts(serverData, testName);

            // plug into the sk-plot directive
            $scope.plotData = temp.plotData;
            $scope.plotTicks = temp.getTicks.bind(temp);

            // used to render information about tests
            $scope.allAggregates = temp.allAggregates;
            $scope.allTests = temp.testDetails;
            $scope.allParams = ns.getSortedParams(serverData, false);
            updateQueryStr(serverData.query || {});

            $scope.state = 'ready';
          },
          function (errResp) {
            retry();
            console.log("Error:", errResp);
          });
      };

      function retry() {
        $scope.state = 'retry';
        $timeout($scope.loadCounts, $scope.reloadInterval * 1000);
      }

      function updateQueryStr(newQuery) {
        $scope.query = newQuery;
        $location.search(newQuery);
        $scope.qStr = ns.extractQueryString($location.url());
      }

      // initialize the members and load the data.
      $scope.reloadInterval = 3;
      $scope.allTests = [];
      $scope.plotData = [];
      $scope.plotTicks = null;
      $scope.oneTest = !!testName;
      $scope.allParams = [];
      updateQueryStr($location.search());
      $scope.loadCounts();
    }]);


  /*
   * TriageCtrl is the controller for the macro triage view. It manages the UI
   * and backend requests. Processing is delegated to the functions in the
   * 'skia' namespace (implemented in logic.js).
   */
  app.controller('TriageCtrl', ['$scope', '$routeParams', '$location', '$timeout', 'dataService',
    function($scope, $routeParams, $location, $timeout, dataService) {
      var triageStateManager;
      var completeTestSet, untriagedTestsOnly;
      // var flagsQuery = {};
      var blameLists = {};
      var allCommits = [];

      var sortFn = function(a,b) {
        return (a.name === b.name) ? 0 : (a.name < b.name) ? -1 : 1;
      };

      function processServerData(promise) {
        promise.then(
          function (serverData) {
            if (!serverData) {
              retry();
              return serverData;
            };

            // Get all the triage data and sort the tests.
            var temp = ns.extractTriageData(serverData, true);
            temp.tests.sort(sortFn);

            triageStateManager.setNewState(temp.triageState);

            // Build the blamelist for quick lookup.
            blameLists = {};
            if (serverData.blames) {
              for(var k in serverData.blames) {
                if (serverData.blames.hasOwnProperty(k)) {
                  var blameObj = {};
                  for(var i=0, len=serverData.blames[k].length; i<len; i++) {
                    blameObj[serverData.blames[k][i].digest] = serverData.blames[k][i];
                  }
                  blameLists[k] = blameObj;
                }
              }
            }

            allCommits = serverData.commits;

            completeTestSet = temp.tests;
            untriagedTestsOnly = temp.tests.filter(
              function(t) { return t.untStats.unique > 0; });
            $scope.updateContent();

            $scope.allParams = temp.allParams;
            $scope.crLinks =temp.commitRanges;
            setQuery(serverData.query || {});

            $scope.displayQuery = serverData.query || {};
            $scope.selectionInfo = {
              untStats: temp.untStats,
              posStats: temp.posStats,
              negStats: temp.negStats
            }

            $scope.state = 'ready';
            $scope.checkStatus();
          },
          function (errResp) {
            retry();
            console.log("Error:", errResp);
            $scope.checkStatus();
          });
      }

      $scope.getBlames = function (oneTest) {
        var ret = [];
        var uniques = {};
        var digests = (oneTest.collapsed) ?
              oneTest.untriaged.map(function(x) { return x.digest; }) :
              [oneTest.untriaged.digest];

        for(var dIdx=0, dLen=digests.length; dIdx < dLen; dIdx++) {
          var arr = blameLists[oneTest.name][digests[dIdx]].freq;
          var commitsRange = allCommits.slice(-arr.length)
          var maxVal = Math.max.apply(null, arr)
          for(var i=0, len=arr.length; i<len; i++) {
            if ((arr[i] == maxVal) && (!uniques[commitsRange[i].author])) {
              ret.push(commitsRange[i].author);
              uniques[commitsRange[i].author]=true;
            }
          }
        }
        return ret;
      };

      // Load triage data across tests.
      $scope.loadAllTriageData = function () {
        $scope.state = 'loading';
        processServerData(dataService.loadData(ns.c.URL_TRIAGE, getCombinedQuery()), true);
      };

      $scope.saveTriageState = function() {
        $scope.state = 'saving';
        triageStateManager.saveTriageState(function (promise) {
          promise.then(
            function (resp) {
              $scope.loadAllTriageData();
            },
            function (errResp) {
              console.log(errResp);
            });
        });
      };

      function retry() {
        $scope.state = 'retry';
        $timeout($scope.loadCounts, $scope.reloadInterval * 1000);
      }

      $scope.selectCommitRange = function(commitRange) {
        if (commitRange) {
          $scope.commitRangeQuery[ns.c.QUERY_COMMIT_START] = commitRange.start.hash;
          $scope.commitRangeQuery[ns.c.QUERY_COMMIT_END] = '';
        }
        $scope.loadAllTriageData();
      };

      $scope.resetCommitRangeQuery = function () {
        $scope.commitRangeQuery = angular.copy($scope.originalCommitRange);
      };

      function setQuery(newQuery) {
        var q = ns.splitQuery(newQuery, $scope.allParams);
        $scope.filteredQuery = q.paramQuery;
        $scope.commitRangeQuery = q.commitRangeQuery;
        $scope.originalCommitRange = angular.copy(q.commitRangeQuery);
        $scope.flagsQuery = q.flagsQuery;

        $scope.headSelected = ns.flatQueryValueOr($scope.flagsQuery, ns.c.QUERY_HEAD, '0');

        $location.search(newQuery);
        $scope.qStr = ns.extractQueryString($location.url());
      }

      function getCombinedQuery() {
        var result = ns.unionObject($scope.filteredQuery,
                                    $scope.commitRangeQuery, $scope.flagsQuery);
        return result;
      }

      $scope.$watch('commitRangeQuery', function() {
        $scope.crClean = angular.equals($scope.commitRangeQuery, $scope.originalCommitRange);
      }, true);

      // Pick the subset of content we are interested in.
      $scope.updateContent = function () {
        var t = ($scope.showUntriagedOnly) ? untriagedTestsOnly : completeTestSet;
        $scope.paginator.setContent(t, $scope.collapseTests);
      };

      $scope.getImageLists = function(oneTest) {
        if (oneTest.untStats.unique > 0) {
          pos = oneTest.untriaged.diffs;
        }

        return {
          positive: pos,
          negative: neg,
          untriaged: unt
        };
      }

      // unroll converts the tests with nested untriaged digests into a flat
      // list.
      function unroll(tests, collapse) {
        var result = [];
        var rs, pos, diff;

        if (collapse) {
          for(var i=0, len=tests.length; i<len; i++) {
              result.push({
                name: tests[i].name,
                showName: true,
                collapsed: true,
                posUnique: tests[i].posStats.unique,
                negUnique: tests[i].negStats.unique,
                untUnique: tests[i].untStats.unique,
                pos: null,
                untriaged: tests[i].untriaged,
                diff: null,
              });
          }
        } else {
          for(var i=0, len=tests.length; i<len; i++) {
            rs = (tests[i].untStats.unique > 0) ? tests[i].untStats.unique : 1;
            for(var j=0; j < rs; j++) {
              pos = (tests[i].untriaged[j] && tests[i].untriaged[j].diffs[0]) ? tests[i].positiveMap[tests[i].untriaged[j].diffs[0].posDigest] : false;
              diff = (tests[i].untriaged[j]) ? tests[i].untriaged[j].diffs[0] : false;
              result.push({
                name: tests[i].name,
                showName: j === 0,
                collapsed: false,
                posUnique: tests[i].posStats.unique,
                negUnique: tests[i].negStats.unique,
                untUnique: tests[i].untStats.unique,
                pos: pos,
                untriaged: tests[i].untriaged[j],
                diff: diff
              });
            }
          }
        }

        return result;
      };

      $scope.headChanged = function() {
        $scope.flagsQuery[ns.c.QUERY_HEAD] = [$scope.headSelected];
        $scope.loadAllTriageData();
      };

      // initialize the members and load the data.
      function init() {
        // Initialize the triage state manager.
        triageStateManager = new TriageStateManager($scope, dataService);

        $scope.reloadInterval = 3;
        $scope.allTests = [];
        $scope.allParams = [];
        $scope.crLinks = [];
        $scope.imageSize = 100;

        $scope.showUntriagedOnly = true;
        $scope.collapseTests = true;
        $scope.pageSize = 50;
        $scope.paginator = new Paginator($scope.pageSize, 5, paginateTests, function(page) {
          $scope.allTests = unroll(page, $scope.collapseTests);
        });

        // Inject the constants into the scope.
        $scope.c = ns.c;

        setQuery($location.search());
        $scope.loadAllTriageData();
      };
      init();

      // Register for corpus changes.
      $scope.$on('corpus-change', function() {
        init();
      });
    }]);

  /**
  * Class TriageStateManager wraps around a scope and handles changes in
  * the triage state.
  */
  function TriageStateManager($scope, dataService) {
    // Keep a reference to the scope and initialize it.
    this.$scope = $scope;
    this.dataService = dataService;
    this.$scope.initialTriageState = {};
    this.$scope.triageState = {};
    this.$scope.triageStateDelta = {};
    this.updatedTriageState(null);

    // Inject the functions into the scope. Make sure to bind the object.
    this.$scope.setTriageState = this.setTriageState.bind(this);
    this.$scope.resetTriageState = this.resetTriageState.bind(this);
  }

  TriageStateManager.prototype.setNewState = function(triageState) {
    this.$scope.initialTriageState = triageState;
    this.$scope.triageState = angular.copy(triageState);
    this.updatedTriageState();
  };

  /** updatedTriageState checks whether the currently assigned labels
  * have changed. This is used to enable/disable the save and reset
  * buttons among other things.
  */
  TriageStateManager.prototype.updatedTriageState = function(testName) {
    this.$scope.triageStateDirty =
                  !angular.equals(this.$scope.triageState, this.$scope.initialTriageState);
    this.$scope.pendingCount = ns.updateDelta(this.$scope.triageState,
      this.$scope.initialTriageState, this.$scope.triageStateDelta, testName);
  };

  // setTriageState sets the label of the given untriaged digest.
  // If changeData is an array it will set the labels for all digests in the array.
  TriageStateManager.prototype.setTriageState= function (testName, changeData, value) {
    if (changeData.constructor === Array) {
      for(var i=0, len=changeData.length; i < len; i++) {
        this.$scope.triageState[testName][changeData[i].digest] = value;
      }
    } else if (typeof(changeData) === 'object') {
      for(var testName in changeData) {
        if (changeData.hasOwnProperty(testName)) {
          for (var digest in changeData[testName]) {
            if (changeData[testName].hasOwnProperty(digest)) {
              this.$scope.triageState[testName][digest] = value;
            }
          }
        }
      }
      testName = null;
    } else {
      this.$scope.triageState[testName][changeData] = value;
    }
    this.updatedTriageState(testName);
  };

  // resetTriageState clears all labels of the untriaged digests.
  TriageStateManager.prototype.resetTriageState = function () {
    this.$scope.triageState = angular.copy(this.$scope.initialTriageState);
    this.updatedTriageState();
  };

  // saveTriageState saves the labeled untriaged digest to the backend
  // and retrieves the new triage state for this test.
  TriageStateManager.prototype.saveTriageState = function (responseFn) {
    var req = new ns.TriageDigestReq();
    for(var testName in this.$scope.triageStateDelta) {
      if (this.$scope.triageStateDelta.hasOwnProperty(testName)) {
        var delta = this.$scope.triageStateDelta[testName].delta;
        for(var digest in delta) {
          if (delta.hasOwnProperty(digest)) {
            req.add(testName, digest, this.$scope.triageState[testName][digest]);
          }
        }
      }
    }
    responseFn(this.dataService.sendData(ns.c.URL_TRIAGE, req));
  };

  // paginateTests splits the array of tests into pages with the given
  // pageSize. Each untriaged digest in a test is considered an entry that
  // counts towards the pagesize. Each page will contain at least one complete
  // test. The number of images might be larger or smaller than page size as
  // a result.
  function paginateTests(pageSize, tests, collapse) {
    var pages = [];
    var current =[];
    var currentCounter = 0, incVal;

    if (collapse) {
      for(var i=0, len=tests.length; i < len; i += pageSize) {
        pages.push(tests.slice(i, i+pageSize));
      }
    } else {
      for(var i=0, len=tests.length; i < len;) {
        incVal = tests[i].untStats.unique > 0 ? tests[i].untStats.unique : 1;
        current.push(tests[i]);
        currentCounter += incVal;
        i++;

        while(i < len) {
          incVal = tests[i].untStats.unique > 0 ? tests[i].untStats.unique : 1;
          if ((currentCounter + incVal) > pageSize) {
            break;
          }
          current.push(tests[i]);
          currentCounter += incVal;
          i++;
        }
        pages.push(current);
        current = [];
        currentCounter = 0;
      }
    }

    return pages;
  };

  // Paginator is a helper class that manages pagination state.
  function Paginator(pageSize, rangeLen, paginationFn, callback) {
    this.pageSize = pageSize;
    this.rangeLen = rangeLen;
    this.callback = callback;
    this.paginationFn = paginationFn;
    this.len = 0;
    this.rangeOffset = Math.floor(this.rangeLen/2);
  }

  Paginator.prototype.setContent = function( _args_ ) {
    var args = [this.pageSize];
    for(var i =0; i < arguments.length; i++) {
      args.push(arguments[i]);
    }
    this.pages = this.paginationFn.apply(null, args);
    this.len =this.pages.length;
    this.lastIndex = this.pages.length-1;
    this.current = -1;
    this.gotoPage(0);
  };

  Paginator.prototype.gotoPage = function(targetPage) {
    if ((this.len == 0) || ((targetPage >= 0) && (targetPage <= this.lastIndex))) {
      this.current = targetPage;
      this.first = this.current === 0;
      this.last = this.current === this.lastIndex;
      this.recalcRange();
      this.currentPage = this.pages[this.current] || [];
      this.callback(this.currentPage);
    };
  };

  Paginator.prototype.recalcRange = function() {
    var start = Math.max(Math.min(this.current - this.rangeOffset, this.len-this.rangeLen), 0);
    var end = Math.min(start + this.rangeLen - 1, this.lastIndex);
    this.range = [];
    for(var i=start; i <= end; i++) {
      this.range.push(i);
    }
    this.rangeStart = this.range[0];
  };

  Paginator.prototype.gotoPrevious = function() {
    if (this.current > 0) {
      this.gotoPage(this.current-1);
    }
  };

  Paginator.prototype.gotoNext = function() {
    if (this.current < this.lastIndex) {
      this.gotoPage(this.current+1);
    }
  };

  Paginator.prototype.gotoFirst = function() {
    this.gotoPage(0);
  };

  Paginator.prototype.gotoLast = function() {
    this.gotoPage(this.lastIndex);
  };

  /*
   * TriageDetailsCtrl is the controller for the micro triage view. It manages the UI
   * and backend requests. Processing is delegated to the functions in the
   * 'skia' namespace (implemented in logic.js).
   */
  app.controller('TriageDetailsCtrl', ['$scope', '$routeParams', '$location', '$timeout', 'dataService',
    function($scope, $routeParams, $location, $timeout, dataService) {
      // Get the path and use it for the backend request
      var testName = $routeParams.id;
      var path = $location.path();
      var positives, negatives;
      var posIndex, negIndex;
      var triageStateManager;
      var commitsMap, allCommits;
      var blameLists;

      // processServerData is called by loadTriageState and also saveTriageState
      // to process the triage data returned by the server.
      function processServerData(promise, updateQuery) {
        promise.then(
          function (serverData) {
            if (!serverData) {
              retry();
              return serverData;
            };

            var triageData = ns.extractTriageData(serverData, true);
            var testData = triageData.tests[0];
            $scope.untStats = testData.untStats;
            $scope.posStats = testData.posStats;
            $scope.negStats = testData.negStats;

            // TODO(stephana): Factor commits handling into a separate REST endpoint.
            commitsMap = serverData.commitsByDigest && serverData.commitsByDigest[testName] || {};
            allCommits = serverData.commits;

            blameLists = {};
            for(var i=0, len=serverData.blames[testName].length; i<len; i++) {
              blameLists[serverData.blames[testName][i].digest] = serverData.blames[testName][i].freq;
            }

            $scope.untriaged = testData.untriaged;
            triageStateManager.setNewState(triageData.triageState);

            $scope.untIndex = -1;
            $scope.leftIndex = -1;
            posIndex = (testData.positive.length > 0) ? 0 : -1;
            negIndex = (testData.negative.length > 0) ? 0 : -1;
            $scope.showPositives = true;
            $scope.selectUntriaged(0);

            // If there are no untriaged we need to just set the positive
            // values since they are no longer a function of the untriaged.
            if ($scope.untriaged.length === 0) {
              positives = testData.positive;
            };

            negatives = testData.negative;

            // Force the left column to positives since they are initialized.
            $scope.switchLeftColumn(true);
            $scope.allParams = triageData.allParams;

            // Show the source images if there are no positives.
            $scope.showSrcImages = $scope.currentUntriaged &&
                                   !$scope.currentLeft;

            if (updateQuery) {
              $scope.query = serverData.query || {};
              $location.search($scope.query);
            }
            $scope.state='ready';
          },
          function (errResp) {
            retry();
            console.log("Error:", errResp);
          });
      }

      $scope.saveTriageState = function() {
        $scope.state = 'saving';
        triageStateManager.saveTriageState(function (promise) {
          promise.then(
            function(posResponse) {
              $scope.loadTriageData();
            },
            function (errResp) {
              console.log("Error. Save failed:", errResp);
            });
        });
      };




      // loadTriageData sends a GET request to the backend to get the
      // untriaged digests.
      $scope.loadTriageData = function () {
        $scope.state = 'loading';
        processServerData(dataService.loadData(path, $scope.query), true);
      };

      function retry() {
        $scope.state = 'retry';
        $timeout($scope.loadTriageData, $scope.reloadInterval * 1000);
      }

      function getCommitsList(digest, all) {
        if (!blameLists || !blameLists[digest]) {
          return null;
        }

        var commits = [];
        var blameCommits = allCommits.slice(allCommits.length-blameLists[digest].length);
        for(var i=0, len=blameLists[digest].length; i < len; i++) {
          if (!all && blameLists[digest][i]===0) {
            break;
          }

          commits.push({
            hash: blameCommits[i].hash,
            count: blameLists[digest][i],
            commit_time: blameCommits[i].commit_time,
            author: blameCommits[i].author
          });
        }

        return {
          commits: commits,
          hasMore: commits.length !== blameLists[digest].length
        };
      }

      $scope.expandCommitsList = function () {
        if ($scope.commitsList && $scope.commitsList.hasMore) {
          $scope.commitsList = getCommitsList($scope.currentUntriaged.digest, true);
        }
      };

      // selectUntriaged is a UI function to pick an untriaged digest.
      $scope.selectUntriaged = function (idx) {
        if ($scope.untriaged.length === 0) {
          $scope.untIndex = -1;
          $scope.currentUntriaged = null;
        }
        else {
          $scope.currentUntriaged = $scope.untriaged[idx];
          positives =
                  ns.getSortedPositivesFromUntriaged($scope.currentUntriaged);
          $scope.untIndex = idx;
          $scope.commitsList = getCommitsList($scope.currentUntriaged.digest, false);
          setPosIndex(0);
        }
      };

      // selectLeft is a UI function to pick a positive digest.
      $scope.selectLeft = function (idx) {
        if ($scope.leftDigests.length === 0) {
          $scope.currentLeft = null;
          $scope.leftIndex = -1;
        } else {
          $scope.currentLeft = $scope.leftDigests[idx];
          $scope.leftIndex = idx;
        }
      };

      // setPosIndex changes the currently selected positive image whether
      // it is displayed or not.
      function setPosIndex(idx) {
        posIndex = idx;
        if ($scope.showPositives) {
          $scope.leftDigests = positives;
          $scope.selectLeft(posIndex);
        }
      };

      // filterByParams adds the given parameter to the current query and
      // triggers a refresh with the newly set filter.
      $scope.filterByParam = function(param, value, add) {
        if (add) {
          $scope.query[param] = $scope.query[param] || [];
          if ($scope.query[param].indexOf(value) === -1) {
            $scope.query[param].push(value);
          }
        } else {
          $scope.query = {};
          $scope.query[param] = value;
        }
        $scope.loadTriageData();
      };

      // switchLeftColumn changes which set of digests (positive/negative) is
      // displayed in the left hand column.
      $scope.switchLeftColumn = function (forcePositive) {
        if (!$scope.showPositives || forcePositive) {
          $scope.leftDigests = positives;
          negIndex = (forcePositive) ? 0 : $scope.leftIndex;
          $scope.showPositives = true;
          $scope.selectLeft(posIndex);
        } else {
          $scope.leftDigests = negatives;
          posIndex = $scope.leftIndex;
          $scope.showPositives = false;
          $scope.selectLeft(negIndex);
        }
      };

      // getOverlayStyle returns style values that are used to turn the
      // image overlay on and off.
      $scope.getOverlayStyle = function () {
        if ($scope.showOverlay && $scope.currentUntriaged) {
          return {
            backGround: {
              'background-image':
                        "url('" + $scope.currentUntriaged.imgUrl + "')",
              padding:0,
              'background-repeat': 'no-repeat',
              'background-size': 'cover'
            },
            foreGround: {
              padding: 0,
              'background-blend-mode': 'multiply',
              opacity: 0.9,
              border: 0
            }
          };
        }

        return {};
      };

      function init() {
        // Initialize the triage state manager.
        triageStateManager = new TriageStateManager($scope, dataService);

        // Initialize the variables in $scope.
        $scope.testName = testName;
        $scope.untriaged = [];
        $scope.leftDigests = [];
        positives = [];
        negatives = [];
        posIndex = -1;
        negIndex = -1;
        $scope.reloadInterval = 3;

        $scope.posStats = {};

        $scope.negStats = {};

        $scope.posStats = null;
        $scope.negStats = null;
        $scope.untStats = null;

        // Update the derived data.
        $scope.selectUntriaged(0);
        $scope.selectLeft(0);
        $scope.showPositives = true;
        $scope.showOverlay = true;

        // Expose the constants in the template.
        $scope.c = ns.c;

        // Manage the URL query.
        $scope.allParams = [];
        $scope.query = $location.search();

        // Load the data.
        $scope.loadTriageData();
      };
      init();
  }]);

  /**
  * skFlot implements a custom directive around Flot. We are using this
  * directive as an attribute so we can set the size on the div.
  **/
  app.directive('skFlot', ['$window', function($window) {
    var linkFn = function ($scope, element, attrs) {
      var plotObj = new ns.Plot(element);

      function refreshData() {
        plotObj.setData($scope.data, $scope.ticks);
      }

      // watch the data and the ticks and redraw them as needed.
      $scope.$watch('data', refreshData);
      $scope.$watch('ticks', refreshData);

      angular.element($window).bind('resize', function () {
        plotObj.redraw();
      });
    };

    return {
        restrict: 'A',
        replace: false,
        scope: {
            data: '=data',
            ticks: '=ticks'
        },
        // templateUrl: 'dirtemplates/skflot.html',
        link: linkFn
    };
  }]);

  /**
  * skPendingTriage implements a custom directive to provide controls to
  * trigger a triage update.
  **/
  app.directive('skPendingTriage', [ '$timeout', function($timeout) {
    var linkFn = function ($scope, element, attrs) {
      $timeout(function() {
        $scope.resetTriageState = $scope.resetTriageState();
        $scope.saveTriageState = $scope.saveTriageState();
      })
    };

    return {
        restrict: 'E',
        replace: false,
        scope: {
          pendingCount: '=changeCount',
          triageStateDirty: '=dirty',
          resetTriageState: '&resetClick',
          saveTriageState: '&saveClick',
          isLoggedIn: '=loggedIn'
        },
        templateUrl: 'templates/pending-triage.html',
        link: linkFn
    };
  }]);


  /**
  * skQuery implements a custom directive to select parameter values used
  * to query the backend.
  **/
  app.directive('skQuery', ['$timeout', function($timeout) {
    var linkFn = function ($scope, element, attrs) {
      $scope.isEmpty = ns.isEmpty;

      $scope.$watch('outerQuery', function(newVal) {
        $scope.query = angular.copy($scope.outerQuery);
      });

      $scope.$watch('query', function() {
        for(var k in $scope.query) {
          if ($scope.query.hasOwnProperty(k) && ($scope.query[k].length === 0)) {
            delete $scope.query[k];
          }
        }
      }, true);

      $scope.isClean = function() {
        return angular.equals($scope.query, $scope.outerQuery);
      };

      $scope.triggerUpdate = function() {
        $scope.outerQuery = $scope.query;
        // The timeout call is necessary to let outerQuery propagate.
        $timeout(function(){
          $scope.clickUpdate();
          $scope.showFilter = false;
        });
      };
    };

    return {
        restrict: 'E',
        replace: false,
        scope: {
            params: '=allParams',
            outerQuery: '=query',
            clickUpdate: "&clickUpdate"
        },
        templateUrl: 'templates/query.html',
        link: linkFn
    };

  }]);

  /**
  * skParams implements a directive to show the tables in a parameter. If the
  * params-two attributes is supplied it will show both parameter sets in
  * one table.
  **/
  app.directive('skParams', [function() {
    var linkFn = function ($scope, element, attrs) {
      function calcColumns() {
        $scope.params = ns.getCombinedParamsTable($scope.data, $scope.dataTwo);
      }

      $scope.$watch("data", calcColumns);
      $scope.$watch("dataTwo", calcColumns);
    };

    return {
        restrict: 'E',
        replace: true,
        scope: {
            data: '=params',
            dataTwo: '=paramsTwo',
            filterFn: '=filter'
        },
        templateUrl: 'templates/params-table.html',
        link: linkFn
    };

  }]);

  /**
  * skImgContainer implements a directive to wrap an image with a
  * classification label.
  **/
  app.directive('skImgContainer', [function() {
    var linkFn = function ($scope, element, attrs) {
        // Only wire these up if there was a digest given.
        if ($scope.digest) {
          // toggleStateIndicator changes the state of a digest to the 'next' state.
          // This allows to iterate through all states by repeatedly clicking on
          // the indicator.
          $scope.toggleStateIndicator = function (digest) {
            var nextState = ns.nextState($scope.triageState[$scope.testName][digest]);
            $scope.setTriageState($scope.testName, digest, nextState);
          };

          // stateChanged helps the UI figure out if a digest has changed. This is
          // used to change the background of a state indicator.
          $scope.stateChanged = function (digest) {
            return $scope.triageState[$scope.testName][$scope.digest] !==
                   $scope.initialTriageState[$scope.testName][$scope.digest];
          };
        }
        $scope.c = ns.c;
    };

    return {
        restrict: 'E',
        replace: false,
        scope: {
          "digest": "=digest",
          "setTriageState": "=setter",
          "triageState": "=triageState",
          "initialTriageState": "=initialTriageState",
          "imgUrl": "=imgUrl",
          "testName": "=testName"
        },
        transclude: true,
        templateUrl: 'templates/triage-img-container.html',
        link: linkFn
    };
  }]);

  /**
  * skBulkChange implements a directive to bulk change classifications.
  **/
  app.directive('skBulkTriage', ['$timeout', function($timeout) {
    var linkFn = function ($scope, element, attrs) {
      function checkChangeForOneTest() {
        if ($scope.digests.length > 0) {
          $scope.selected = $scope.triageState[$scope.testName][$scope.digests[0].digest];
          for(var i=1, len=$scope.digests.length; i<len; i++) {
             if ($scope.triageState[$scope.testName][$scope.digests[i].digest]
                 !== $scope.selected) {
               $scope.selected = null;
               break;
             }
          }
        } else {
          $scope.selected = null;
        }
      }

      function checkChangeForAllTests() {
        var init = true;
        $scope.selected = null;

        loop:
        for(var k in $scope.triageState) {
          if ($scope.triageState.hasOwnProperty(k)) {
            for(var digest in $scope.triageState[k]) {
              if ($scope.triageState[k].hasOwnProperty(digest)) {
                if (init) {
                  init = false;
                  $scope.selected = $scope.triageState[k][digest];
                }

                if ($scope.triageState[k][digest] !== $scope.selected) {
                  $scope.selected = null;
                  break loop;
                }
              }
            }
          }
        }
      }

      $scope.c = ns.c;
      $timeout(function () {
        $scope.setTriageState = $scope.setTriageState();
        $scope.testName = $scope.testName();
        if ($scope.testName) {
          $scope.selectAll = $scope.setTriageState;
          $scope.$watch('triageState', checkChangeForOneTest, true);
          $scope.$watch('digests', checkChangeForOneTest, true);
        } else {
          $scope.selectAll = function(testName, digests, targetState) {
              $scope.setTriageState(testName, $scope.triageState, targetState);
          };
          $scope.$watch('triageState', checkChangeForAllTests, true);
        }
      });
    };

    return {
        restrict: 'E',
        replace: true,
        scope: {
          "digests": "=digests",
          "setTriageState": "&setter",
          "triageState": "=triageState",
          "testName": "&testName",
          "tests": "=tests"
        },
        transclude: false,
        templateUrl: 'templates/bulk-triage.html',
        link: linkFn
    };
  }]);

  /**
  * skPagination implements a directive to show current pagination state.
  **/
  app.directive('skPagination', [function() {
    return {
        restrict: 'E',
        replace: true,
        scope: {
          "p": "=paginator",
        },
        transclude: false,
        templateUrl: 'templates/pagination.html'
    };
  }]);

  /**
  * skSortedTable shows an object in a sorted table.
  **/
  app.directive('skSortedTable', [function() {
    var linkFn = function ($scope, element, attrs) {
      $scope.$watch('obj', function() {
        $scope.objTable = ns.getSortedObject($scope.obj);
      });
    };

    return {
        restrict: 'E',
        replace: true,
        scope: {
          "obj": "=obj",
          "hOne": "@hOne",
          "hTwo": "@hTwo",
          "empty": "@msg"
        },
        transclude: false,
        templateUrl: 'templates/sorted-table.html',
        link: linkFn
    };
  }]);
  /**
  * dataService provides functions to load data from the backend.
  */
  app.factory('dataService', [ '$http', '$rootScope', '$interval', '$location', '$window',
  function ($http, $rootScope, $interval, $location, $window) {
    // Inject the logoutURL into the rootScope.
    // $rootScope.logoutURL = ns.c.PREFIX_URL + ns.c.URL_LOGOUT;
    $rootScope.getLogoutURL = function() {
      return encodeURI(ns.c.PREFIX_URL +
        ns.c.URL_LOGOUT + '?redirect=' + '/#' + $location.url());
    }

    $rootScope.globalStatus = null;
    $rootScope.checkStatus = function () {
      loadData(ns.c.URL_STATUS).then(
        function (resultResp) {
          $rootScope.globalStatus = resultResp;
          $rootScope.corpStatus = (resultResp && resultResp.corpStatus) || {};
          $rootScope.corpusList = ns.sortedKeys($rootScope.corpStatus);
          if (!$rootScope.corpStatus[$rootScope.currentCorpus] &&
              ($rootScope.corpusList.length > 0)) {
              $rootScope.currentCorpus = $rootScope.corpusList[0];
              $rootScope.corpusChanged();
          }
        },
        function (errorResp) {
          console.log("Got error response for status:", errorResp);
        });
    };

    $rootScope.getStatusString = function(corpus) {
      var status = $rootScope.corpStatus[corpus].ok ?
          'ok' : $rootScope.corpStatus[corpus].untriagedCount +'/'+
                 $rootScope.corpStatus[corpus].negativeCount;
      return corpus + '   -   ' +status;

    };

    $rootScope.corpusList = [];
    $rootScope.defaultShowCorpora = ['gm', 'skp'];

    $rootScope.corpusChanged = function () {
      if (localStorage) {
        localStorage.setItem('corpus', $rootScope.currentCorpus);
      }
      $rootScope.showCorpora = angular.copy($rootScope.defaultShowCorpora);
      if ($rootScope.showCorpora.indexOf($rootScope.currentCorpus) == -1) {
        $rootScope.showCorpora.unshift($rootScope.currentCorpus);
      }
      $rootScope.$broadcast('corpus-change');
    };

    $rootScope.forceTriage = function(corpus) {
      $rootScope.currentCorpus = corpus;
      $location.path("/triage");
      $rootScope.corpusChanged();
    }

    // Load the status every 3 seconds.
    $rootScope.checkStatus();
    $interval($rootScope.checkStatus, 3000);

    // Retrieve the corpus from localStorage if possible.
    var localStorage = $window.localStorage;
    $rootScope.currentCorpus = localStorage && localStorage.getItem('corpus');
    if ($rootScope.currentCorpus) {
      $rootScope.corpusChanged();
    }

    /**
     * @param {string} testName if not null this will cause to fetch counts
     *                          for a specific test.
     * @param {object} query Query string to send to the backend.
     * @return {Promise} Will resolve to either the data (success) or to
     *                   the HTTP response (error).
     **/
    function loadData(path, query) {
      var url = ns.c.PREFIX_URL + path;
      // Inject the corpus selector if there is a query.
      if (query && $rootScope.currentCorpus) {
        query[ns.c.CORPUS_FIELD] = [$rootScope.currentCorpus];
      };

      return httpReq(url, 'GET', null, query).then(
          function(successResp) {
            var ret = successResp.data;
            // Remove the corpus selector if there is a query.
            if (ret && ret.query && ret.query[ns.c.CORPUS_FIELD]) {
              delete ret.query[ns.c.CORPUS_FIELD];
            }
            return ret;
          });
    }

    /**
    * sendData sends the given data to the URL path on the backend via a
    * POST request.
    *
    * @param {string} path relative path. The standard prefix will be added.
    * @param {object} data object to be send as JSON string.
    * @return {Promise} will either resolve to data (success) or an HTTP
    *         response object (error).
    */
    function sendData(path, data) {
      var url = ns.c.PREFIX_URL + path;
      return httpReq(url, 'POST', data).then(
        function(successResp) {
          return successResp.data;
        });
    }

    /**
    * setGlobalLoginStatus injects the login status into the $rootScope
    * so it is accessible throughout the application.
    */
    function setGlobalLoginStatus(userId, loginURL) {
      $rootScope.isLoggedIn = (userId && userId !== '');
      $rootScope.userId = userId;
      $rootScope.loginURL = loginURL;
    }

    /**
    * pollLoginStatus sends a single polling request to the backend
    * to determine whether the user is logged in or not.
    * This corresponds to the backend implementation in 'go/login/login.go'.
    */
    function pollLoginStatus() {
      var url = ns.c.PREFIX_URL + ns.c.URL_LOGIN_STATUS;
      httpReq(url).then(
        function (posResp) {
          setGlobalLoginStatus(posResp.Email, posResp.LoginURL);
        },
        function (errResp) {
          console.log("Error:", errResp);
          setGlobalLoginStatus(null, null);
        });
    }

    // Fetch the login status immediately.
    pollLoginStatus();

    /**
     * Make a HTTP request with the given data, method and query parameters.
     *
     * @param {string} url
     * @param {string} method
     * @param {object} data
     * @param {object} queryParams
     *
     * @return {Promise} promise.
     **/
    function httpReq(url, method, data, queryParams) {
      method = (method) ? method : 'GET';
      var reqConfig = {
        method: method,
        url: url,
        params: queryParams,
        data: data
      };

      return $http(reqConfig).then(
        function(successResp) {
          return successResp.data;
        });
    }


    // Interface of the service:
    return {
      loadData: loadData,
      sendData: sendData
    };

  }]);

})(skia);
