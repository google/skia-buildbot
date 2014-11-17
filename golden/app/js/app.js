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
    $routeProvider.when(ns.c.URL_COUNTS + '/:id?', {templateUrl: 'partials/counts-view.html', controller: 'CountsCtrl'});
    $routeProvider.when(ns.c.URL_TRIAGE + '/:id', {templateUrl: 'partials/triage-view.html',  controller: 'TriageCtrl'});
    $routeProvider.otherwise({redirectTo: ns.c.URL_COUNTS });
  }]);

  /*
   * CountsCtrl controlls the UI on the main view where an overview of
   * test results is presented.
   */
  app.controller('CountsCtrl', ['$scope', '$routeParams', 'dataService', function($scope, $routeParams, dataService) {
    // Get the path and use it for the backend request
    var testName = ($routeParams.id && ($routeParams.id !== '')) ?
                    $routeParams.id : null;

    // Load counts for all tests of a tile.
    function loadCounts() {
      dataService.loadData(ns.c.URL_COUNTS).then(
        function (serverData) {
          var temp = ns.processCounts(serverData, testName);

          // plug into the sk-plot directive
          $scope.plotData = temp.plotData;
          $scope.plotTicks = temp.getTicks.bind(temp);

          // used to render information about tests
          $scope.allAggregates = temp.allAggregates;
          $scope.allTests = temp.testDetails;
        },
        function (errResp) {
          console.log("Error:", errResp);
        });
    }

    // initialize the members and load the data.
    $scope.allTests = [];
    $scope.plotData = [];
    $scope.plotTicks = null;
    $scope.oneTest = !!testName;
    loadCounts();
  }]);

  /*
   * TriageCtrl is the controller for the triage view. It manages the UI
   * and backend requests. Processing is delegated to the functions in the
   * 'skia' namespace (implemented in logic.js).
   */
  app.controller('TriageCtrl', ['$scope', '$routeParams', '$location', 'dataService',
    function($scope, $routeParams, $location, dataService) {
      // Get the path and use it for the backend request
      var testName = $routeParams.id;
      var path = $location.path()

      // processServerData is called by loadTriageState and also saveTriageState
      // to process the triage data returned by the server.
      function processServerData(promise) {
        promise.then(
          function (serverData) {
            $scope.untriaged = ns.getUntriagedSorted(serverData, testName);
            $scope.triageState = ns.getNumArray($scope.untriaged.length, ns.c.UNTRIAGED);
            updatedTriageState();
            $scope.selectUntriaged(0);

            // If there are no untriaged we need to just set the positive
            // values since they are no longer a function of the untriaged.
            if ($scope.untriaged.length === 0) {
              $scope.positives = ns.getSortedPositives(serverData, testName);
              $scope.selectPositive(0);
            };
          },
          function (errResp) {
            console.log("Error:", errResp);
          });
      }

      // loadTriageData sends a GET request to the backend to get the
      // untriaged digests.
      function loadTriageData() {
        processServerData(dataService.loadData(path));
      };

      // updatedTriageState checks whether the currently assigned labels
      // have changed. This is used to enable/disable the save and reset
      // buttons among other things.
      function updatedTriageState() {
        $scope.triageStateDirty = false;
        for(var i=0, len=$scope.triageState.length; i<len; i++) {
          if ($scope.triageState[i] !== ns.c.UNTRIAGED) {
            $scope.triageStateDirty = true;
            break;
          }
        }
      }

      // selectUntriaged is a UI function to pick an untriaged digest.
      $scope.selectUntriaged = function (idx) {
        if ($scope.untriaged.length === 0) {
          $scope.untIndex = -1;
          $scope.currentUntriaged = null;
        }
        else if ($scope.untIndex != idx) {
          $scope.currentUntriaged = $scope.untriaged[idx];
          $scope.positives = ns.getSortedPositivesFromUntriaged($scope.currentUntriaged);
          $scope.untIndex = idx;
          $scope.selectPositive(0);
        }
      };

      // selectPositive is a UI function to pick a positive digest.
      $scope.selectPositive = function (idx) {
        if ($scope.positives.length === 0) {
          $scope.currentPositive = null;
          $scope.posIndex = -1;
        } else {
          $scope.currentPositive = $scope.positives[idx];
          $scope.posIndex = idx;
        }
      };

      // setTriageState sets the label of the given untriaged digest.
      $scope.setTriageState = function (idx, value) {
        $scope.triageState[idx] = value;
        updatedTriageState();
      };

      // resetTriageState clears all labels of the untriaged digests.
      $scope.resetTriageState = function () {
        $scope.triageState = ns.getNumArray($scope.untriaged.length, ns.c.UNTRIAGED);
        updatedTriageState();
      };

      // saveTriageState saves the labeled untriaged digest to the backend
      // and retrieves the new triage state for this test.
      $scope.saveTriageState = function () {
        var req = new ns.TriageDigestReq();
        for (var i=0, len=$scope.triageState.length; i < len; i++) {
          if ($scope.triageState[i] !== ns.c.UNTRIAGED) {
            req.add(testName, $scope.untriaged[i].digest, $scope.triageState[i]);
          }
        }

        processServerData(dataService.sendData(ns.c.URL_TRIAGE, req));
      };

      // Initialize the variables in $scope.
      $scope.untriaged = [];
      $scope.positives = [];
      $scope.triageState = [];

      // Update the derived data.
      updatedTriageState();
      $scope.selectUntriaged(0);
      $scope.selectPositive(0);

      // Expose the constants in the template.
      $scope.c = ns.c;

      // Load the data.
      loadTriageData();
  }]);

  /**
  * skFlot implements a custom directive around Flot. We are using this directive
  * as an attribute so we can set the size on the div.
  *
  **/
  app.directive('skFlot', [function() {
    var linkFn = function ($scope, element, attrs) {
      var plotObj = new ns.Plot(element);

      function refreshData() {
        plotObj.setData($scope.data, $scope.ticks);
      }

      // watch the data and the ticks and redraw them as needed.
      $scope.$watch('data', refreshData);
      $scope.$watch('ticks', refreshData);
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
  * dataService provides functions to load data from the backend.
  */
  app.factory('dataService', [ '$http', '$rootScope', '$timeout', function ($http, $rootScope, $timeout) {
    // Inject the logoutURL into the rootScope.
    $rootScope.logoutURL = ns.c.PREFIX_URL + ns.URL_LOGOUT;

    /**
     * @param {string} testName if not null this will cause to fetch counts
     *                          for a specific test.
     * @return {Promise} Will resolve to either the data (success) or to
     *                   the HTTP response (error).
     **/
    function loadData(path) {
      var url = ns.c.PREFIX_URL + path;
      return httpReq(url).then(
          function(successResp) {
            return successResp.data;
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
