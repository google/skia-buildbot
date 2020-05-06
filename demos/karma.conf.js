// TODO(lovisolo): Move the exclude list below to common-sk.
module.exports = function(config) {
  commonJsFn = module.exports = require('common-sk/karma.common.js')(__dirname);
  commonJsFn(config);
  config.set({
    exclude: [
      'modules/*_puppeteer_test.js',
      'modules/**/*_puppeteer_test.js',
    ]
  });
};