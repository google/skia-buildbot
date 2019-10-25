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
        from: './config.json',
        to: 'config.json'
      },
    ])
  );
  config.plugins.push(
    new HtmlWebpackInjectAttributesPlugin({
      nonce: "{% .Nonce %}",
    }),
  );
  config.resolve = config.resolve || {};
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  return config;
}
