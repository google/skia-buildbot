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
    $routeProvider.when('/', {templateUrl: 'partials/index-view.html', controller: 'IndexCtrl'});
    $routeProvider.when('/triage', {templateUrl: 'partials/triage-view.html',  controller: 'TriageCtrl'});
    $routeProvider.otherwise({redirectTo: '/'});
  }]);

  /*
   * IndexCtrl controlls the UI on the main view where an overview of
   * test results is presented.
   */
  app.controller('IndexCtrl', ['$scope', 'dataService', function($scope, dataService) {
    // Load counts for all tests of a tile.
    function loadAllTests() {
      dataService.loadCounts().then(
        function (serverData) {
          var temp = ns.processAllCounts(serverData);

          // plug into the sk-plot directive
          $scope.plotData = temp.plotData;
          $scope.plotTicks = temp.plotTicks;

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
    loadAllTests();
  }]);

  /*
   *  TODO (stephana): Placeholder for the controller of the triage view.
   */
  app.controller('TriageCtrl', ['$scope', function($scope) {

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
  app.factory('dataService', [ '$http', function ($http) {

    /**
     * @param {string} testName if not null this will cause to fetch counts
     *                          for a specific test.
     * @return {Promise} Will resolve to either the data (success) or to
     *                   the HTTP response (error).
     **/
    function loadCounts(testName) {
      var url = '/rest/tilecounts';
      if (testName) {
        url += '/' + testName;
      }

      return httpGetData(url).then(
          function(successResp) {
            return successResp.data;
          });
    }

    /**
     * Make a HTTP get request with the given query parameters.
     *
     * @param {string} url
     * @param {object} queryParams
     *
     * @return {Promise} promise.
     **/
    function httpGetData(url, queryParams) {
      var reqConfig = {
        method: 'GET',
        url: url,
        params: queryParams
      };

      return $http(reqConfig).then(
        function(successResp) {
          return successResp.data;
        });
    }

    // Interface of the service:
    return {
      loadCounts: loadCounts
    };

  }]);

})(skia);
