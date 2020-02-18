const commonBuilder = require('pulito');
const HtmlWebpackInjectAttributesPlugin = require('html-webpack-inject-attributes-plugin');
const { resolve } = require('path')

module.exports = (env, argv) => {
  const config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/static/';
  config.plugins.push(
    new HtmlWebpackInjectAttributesPlugin({
      nonce: "{%.nonce%}",
    }),
  );
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  // config.resolve.modules = [resolve(__dirname, 'node_modules')];
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  // config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];

  return config;
}
