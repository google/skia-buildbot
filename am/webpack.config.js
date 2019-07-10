const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackInjectAttributesPlugin = require('html-webpack-inject-attributes-plugin');
const { resolve } = require('path')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/static/';

  config.plugins.push(
    new CopyWebpackPlugin([
      {
        from: './images/icon.png',
        to: 'icon.png'
      },
      {
        from: './images/icon-active.png',
        to: 'icon-active.png'
      }
    ])
  );
  config.plugins.push(
    new HtmlWebpackInjectAttributesPlugin({
      nonce: "{%.nonce%}",
    }),
  );
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  return config;
}
