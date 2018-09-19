const commonBuilder = require('pulito');
const { resolve } = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  // Make all CSS/JS files appear at the /res location.
  config.output.publicPath='/res/';
  config.plugins.push(
    new CopyWebpackPlugin([
        { from: 'node_modules/pathkit-wasm/bin/pathkit.wasm' },
        { from: 'node_modules/@webcomponents/custom-elements/custom-elements.min.js' }
    ])
  );
  config.node = {
    fs: 'empty'
  };
  return config
}
