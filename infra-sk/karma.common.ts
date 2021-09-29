import karma from 'karma';
import webpack from 'webpack';

// Adapted from https://github.com/google/common-sk/blob/master/karma.common.js.
export function setCommonConfigOptions(karmaConfig: karma.Config, webpackConfig: webpack.Configuration) {
  // Work-around for karma-webpack issues:
  // https://github.com/webpack-contrib/karma-webpack/issues/322#issuecomment-417862717
  webpackConfig.output = {
    filename: '[name]',
  };

  karmaConfig.set({
    // base path, that will be used to resolve files and exclude
    basePath: '',

    // frameworks to use (loaded in reverse order, so chai-dom loads after chai)
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
      'modules/*_test.ts',
      'modules/**/*_test.ts',
      'modules/*_test.js',
      'modules/**/*_test.js',
    ],

    preprocessors: {
      // add webpack as preprocessor
      'modules/*_test.ts': ['webpack'],
      'modules/**/*_test.ts': ['webpack'],
      'modules/*_test.js': ['webpack'],
      'modules/**/*_test.js': ['webpack'],
    },

    // list of files to exclude
    exclude: [
      'modules/*_puppeteer_test.ts',
      'modules/**/*_puppeteer_test.ts',
      'modules/*_puppeteer_test.js',
      'modules/**/*_puppeteer_test.js',
      'modules/*_nodejs_test.ts',
      'modules/**/*_nodejs_test.ts',
      'modules/*_nodejs_test.js',
      'modules/**/*_nodejs_test.js',
    ],

    // test results reporter to use
    // possible values: 'dots', 'progress', 'junit', 'growl', 'coverage'
    reporters: ['dots'],

    // Get the port from KARMA_PORT if it is set.
    port: parseInt(process.env.KARMA_PORT || '9876'),

    // enable / disable colors in the output (reporters and logs)
    colors: false,

    // level of logging
    // possible values: karmaConfig.LOG_DISABLE || karmaConfig.LOG_ERROR ||
    // karmaConfig.LOG_WARN || karmaConfig.LOG_INFO || karmaConfig.LOG_DEBUG
    logLevel: karmaConfig.LOG_INFO,

    // enable / disable watching file and executing tests whenever any file changes
    autoWatch: true,

    // Start these browsers.
    browsers: ['Chrome'],

    customLaunchers: {
      ChromeHeadlessCustom: {
        base: 'ChromeHeadless',
        flags: ['--no-sandbox', '--disable-gpu'],
      },
    },

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

    webpackMiddleware: {},
  } as karma.ConfigOptions);
}
