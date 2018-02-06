const HtmlWebpackPlugin = require('html-webpack-plugin');
const CommonBuilder = require('./webpack.common.js');

let common = CommonBuilder(__dirname);
common.entry.demo = './demo/demo.js';
common.output.library = 'Demo';
common.plugins.push(
  new HtmlWebpackPlugin({
    filename: 'demo.html',
    template: './demo/demo.html',
  }),
);

module.exports = common;
