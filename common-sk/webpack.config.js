const { glob } = require('glob');

const commonBuilder = require('pulito');

//const { addCommon } = require('./webpack.common.js');

module.exports = (env, argv) => {
//  let config = addCommon(commonBuilder(env, argv, __dirname));
  let config = commonBuilder(env, argv, __dirname);

  config.entry.tests = glob.sync('./modules/**/*_test.js');
  return config;
}
