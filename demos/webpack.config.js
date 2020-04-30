const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const { resolve } = require('path');

module.exports = (env, argv) => {
  const config = commonBuilder(env, argv, __dirname);
  config.output.publicPath = '/dist/';
  config.resolve = config.resolve || {};
  // https://github.com/webpack/node-libs-browser/issues/26#issuecomment-267954095
  config.resolve.modules = [resolve(__dirname, 'node_modules'), 'node_modules'];
  config.plugins.push(
    new CopyWebpackPlugin([
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.wasm') },
      { from: resolve(__dirname, 'build/canvaskit/canvaskit.js') },
      { from: resolve(__dirname, 'node_modules/@webcomponents/custom-elements/custom-elements.min.js') },
    ]),
  );
  config.node = {
    fs: 'empty',
  };

  return config;
};
