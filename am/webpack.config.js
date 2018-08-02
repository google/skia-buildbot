const commonBuilder = require('pulito');
const CopyWebpackPlugin = require('copy-webpack-plugin')

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
  return config;
}
