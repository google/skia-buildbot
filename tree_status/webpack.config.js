const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackInjectAttributesPlugin = require('html-webpack-inject-attributes-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const { resolve } = require('path')

module.exports = (env, argv) => {
  const config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/static/';

  // Don't minify the HTML since it contains Go template tags.
  config.plugins.forEach((c) => {
    if (c instanceof HtmlWebpackPlugin) {
      c.options.minify = false;
    }
  });

  config.plugins.push(
    new CopyWebpackPlugin([
      {
        from: './images/favicon.ico',
        to: 'favicon.ico'
      },
      {
        from: './images/robocop.jpg',
        to: 'robocop.jpg'
      },
      {
        from: './images/sheriff.jpg',
        to: 'sheriff.jpg'
      },
      {
        from: './images/trooper.jpg',
        to: 'trooper.jpg'
      },
      {
        from: './images/wrangler.jpg',
        to: 'wrangler.jpg'
      }
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
