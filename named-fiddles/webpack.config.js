const commonBuilder = require('pulito');
//const commonBuilder = require('../webpack.common.js');

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/static/';
  return config;
}
