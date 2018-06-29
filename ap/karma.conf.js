const path = require('path');

module.exports = function(config) {

  let webpackConfig = require(path.resolve(__dirname, 'webpack.config.js'));
  // Webpack 3+ configs can be either objects or functions that produce the
  // config object. Karma currently doesn't handle the latter, so do it
  // ourselves here.
  if (typeof webpackConfig === 'function') {
    webpackConfig = webpackConfig({}, {mode: 'development'});
  }
  webpackConfig.entry = null;
  webpackConfig.mode = 'development';

  config.set({

    // base path, that will be used to resolve files and exclude
    basePath: '',


    // frameworks to use
    frameworks: ['mocha', 'chai', 'sinon'],

    plugins: [
      'karma-chrome-launcher',
      'karma-firefox-launcher',
      'karma-webpack',
      'karma-sinon',
      'karma-mocha',
      'karma-chai',
    ],

    // list of files / patterns to load in the browser
    files: [
      'node_modules/@webcomponents/custom-elements/custom-elements.min.js',
      'elements-sk/**/*_test.js',
    ],

    preprocessors: {
      // add webpack as preprocessor
      'elements-sk/**/*_test.js': [ 'webpack' ],
    },

    // list of files to exclude
    exclude: [
      'elements-sk/node_modules/**'
    ],


    // test results reporter to use
    // possible values: 'dots', 'progress', 'junit', 'growl', 'coverage'
    reporters: ['dots'],


    // Get the port from KARMA_PORT if it is set.
    port: parseInt(process.env.KARMA_PORT || "9876"),


    // enable / disable colors in the output (reporters and logs)
    colors: false,


    // level of logging
    // possible values: config.LOG_DISABLE || config.LOG_ERROR ||
    // config.LOG_WARN || config.LOG_INFO || config.LOG_DEBUG
    logLevel: config.LOG_INFO,


    // enable / disable watching file and executing tests whenever any file changes
    autoWatch: true,


    // Start these browsers.
    browsers: ['Chrome', 'Firefox'],


    // If browser does not capture in given timeout [ms], kill it
    captureTimeout: 60000,


    // Continuous Integration mode
    // if true, it capture browsers, run tests and exit
    //
    // This can be over-ridden by command-line flag when running Karma. I.e.:
    //
    //    ./node_modules/karma/bin/karma --no-single-run start
    //
    singleRun: true,

    webpack: webpackConfig,
  });
};
