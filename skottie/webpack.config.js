const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin')

module.exports = (env, argv) => {
  let config = commonBuilder(env, argv, __dirname);
  config.output.publicPath='/static/';
  config.plugins.push(
    new CopyWebpackPlugin([{
      from: './node_modules/lottie-web/build/player/lottie.min.js',
      to: 'lottie.min.js'
    }])
  );
  return config;
}
