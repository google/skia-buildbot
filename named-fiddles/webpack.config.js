const commonBuilder = require('pulito');
const commonsk = require('../common-sk/webpack.common.js');

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/static/';
  config = commonsk(config);
  return config;
}
