const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const HtmlWebpackInjectAttributesPlugin = require('html-webpack-inject-attributes-plugin');
const HtmlWebpackPlugin = require('html-webpack-plugin');
const { resolve } = require('path');

module.exports = (env, argv) => {
  const config = commonBuilder(env, argv, __dirname);
  config.output.publicPath = '/static/';

  // Don't minify the HTML since it contains Go template tags.
  config.plugins.forEach((c) => {
    if (c instanceof HtmlWebpackPlugin) {
      c.options.minify = false;
    }
  });

  config.plugins.push(
    new CopyWebpackPlugin([
      {
        from: resolve(__dirname, 'config.json'),
        to: 'config.json',
      },
    ]),
  );
  config.plugins.push(
    new HtmlWebpackInjectAttributesPlugin({
      nonce: '{% .Nonce %}',
    }),
  );
  config.resolve = config.resolve || {};
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules')];
  return config;
};
