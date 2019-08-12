const commonBuilder = require('pulito');
const { resolve } = require('path')
const webpack = require('webpack');

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/dist/';
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  config.plugins.push(
    // Drop locale files for chart.js.
    // See https://github.com/chartjs/Chart.js/issues/4303#issuecomment-461161063
    new webpack.IgnorePlugin(/^\.\/locale$/, /moment$/)
  );
  return config;
}
