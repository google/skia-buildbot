const HtmlWebpackPlugin = require('html-webpack-plugin');

var common = require('./webpack.common.js');
common.entry.demo = './demo/demo.js';
common.output.library = 'Demo';
common.plugins.push(
  new HtmlWebpackPlugin({
    filename: 'demo.html',
    template: './demo/demo.html',
    chunks: ['demo'],
  }),
);

module.exports = common;
