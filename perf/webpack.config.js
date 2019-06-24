const commonBuilder = require('pulito');

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/dist/';
  return config;
}
