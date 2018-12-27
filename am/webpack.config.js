const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackInjectAttributesPlugin = require('html-webpack-inject-attributes-plugin');

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
  return config;
}
