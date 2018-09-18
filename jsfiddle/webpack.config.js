const commonBuilder = require('pulito');
const { resolve } = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.plugins.push(
    new CopyWebpackPlugin([
        { from: 'node_modules/pathkit-wasm/bin/pathkit.wasm' }
    ])
  );
  config.node = {
    fs: 'empty'
  };
  return config
}
