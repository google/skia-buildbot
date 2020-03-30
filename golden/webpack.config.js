const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const { resolve } = require('path');

module.exports = (env, argv) => {
  const config = commonBuilder(env, argv, __dirname);
  config.output.publicPath = '/dist/';
  config.resolve = config.resolve || {};
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];

  if (argv.mode !== 'production') {
    console.log('serving demo assets');
    config.plugins.push(
      new CopyWebpackPlugin([
        {
          from: 'demo-page-assets',
        },
      ]),
    );
  }

  return config;
};
