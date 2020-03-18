// This is a common configuration for running karma
// tests in your modules, including custom elements.
// To use this, just create a karma.conf.js file in your
// application directory and populate it with:
//
//     module.exports = require('../common-sk/karma.common.js')(__dirname);
//
// Add all the testing packages found in common/packages.json via yarn:
//
//     $ yarn add chai karma karma-chai karma-chrome-launcher \
//       karma-firefox-launcher karma-mocha karma-sinon karma-webpack mocha sinon
//
// Then add this to your Makefile:
//
//     test: build
//         # Run the generated tests just once under Xvfb.
//         xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run
//
// You can always run:
//
//     $ npx karma start
//
// if you need to debug tests using your desktop browser.
const path = require('path');

module.exports = function(dirname) {
  return function(config) {

    let webpackConfig = require(path.resolve(dirname, 'webpack.config.js'));
    // Webpack 3+ configs can be either objects or functions that produce the
    // config object. Karma currently doesn't handle the latter, so do it
    // ourselves here.
    if (typeof webpackConfig === 'function') {
      webpackConfig = webpackConfig({}, {mode: 'development'});
    }
    webpackConfig.entry = null;
    webpackConfig.mode = 'development';

    // Work-around for karma-webpack issues:
    // https://github.com/webpack-contrib/karma-webpack/issues/322#issuecomment-417862717
    webpackConfig.output= {
      filename: '[name]',
    };

    config.set({

      // base path, that will be used to resolve files and exclude
      basePath: '',


      // frameworks to use
      frameworks: ['mocha', 'chai-dom', 'chai', 'sinon'],

      plugins: [
        'karma-chrome-launcher',
        'karma-firefox-launcher',
        'karma-webpack',
        'karma-sinon',
        'karma-mocha',
        'karma-chai',
        'karma-chai-dom',
      ],

      // list of files / patterns to load in the browser
      files: [
        'node_modules/@webcomponents/custom-elements/custom-elements.min.js',
        'modules/*_test.js',
        'modules/**/*_test.js',
      ],

      preprocessors: {
        // add webpack as preprocessor
        'modules/*_test.js': [ 'webpack' ],
        'modules/**/*_test.js': [ 'webpack' ],
      },

      // list of files to exclude
      exclude: [
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
      browsers: ['Chrome'],


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
  }
};
