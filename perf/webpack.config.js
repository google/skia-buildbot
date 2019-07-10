const commonBuilder = require('pulito');
const path = require('path');

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/dist/';
  config.resolve = config.resolve || {};
  config.resolve.modules = [path.resolve(__dirname, 'node_modules')];
  return config;
}
