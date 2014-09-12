'use strict';


/*
 * Wrap everything into an IIFE to not polute the global namespace.
 */
(function () {

  var app = angular.module('rbtApp', ['ngRoute']);

  // Configure the different within app views.
  app.config(['$routeProvider', function($routeProvider) {
    $routeProvider.when('/', {templateUrl: 'partials/index-view.html', controller: 'IndexCtrl'});
    $routeProvider.when('/view', {templateUrl: 'partials/rebaseline-view.html',  controller: 'RebaselineCrtrl'});
    $routeProvider.otherwise({redirectTo: '/'});
  }]);

  /*
   * Index Controller 
   */
  app.controller('IndexCtrl', ['$scope', function($scope) {

  }]);

  /* 
   *  RebaselineCtrl
   */
  app.controller('RebaselineCrtrl', ['$scope', function($scope) {

  }]);

})();
