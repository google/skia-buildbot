const { commonBuilder } = require('../res/mod/webpack.common.js');
const HtmlWebpackPlugin = require('html-webpack-plugin');

let common = commonBuilder(__dirname);
common.entry.index = './pages/index.js'
common.plugins.push(
  new HtmlWebpackPlugin({
    filename: 'index.html',
    template: './pages/index.html',
    chunks: ['index'],
  })
);

module.exports = common
