const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const { resolve } = require('path');

module.exports = (env, argv) => {
  const config = commonBuilder(env, argv, __dirname);
  config.plugins.push(
    new CopyWebpackPlugin([
      {
        from: resolve(__dirname, 'res/img/favicon.ico'),
        to: 'favicon.ico',
      },
    ]),
  );
  config.output.publicPath = '/dist/';
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  return config;
};
