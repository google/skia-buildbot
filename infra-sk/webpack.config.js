const { glob } = require('glob');
const commonBuilder = require('pulito');

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);

  config.entry.tests = glob.sync('./modules/**/*_test.js');
  return config;
}
